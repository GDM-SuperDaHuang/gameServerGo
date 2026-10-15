package rpc

import (
	"context"
	"errors"
	"fmt"
	"gameServer/pkg/bytes"
	"gameServer/pkg/logger/log2"
	"gameServer/protobuf/protoHandlerInit"
	"gameServer/service/common"
	"reflect"
	"strings"

	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

var (
	typeOfContext  = reflect.TypeOf((*context.Context)(nil)).Elem()
	typeOfProtoMsg = reflect.TypeOf((*proto.Message)(nil)).Elem()
	typeOfResult   = reflect.TypeOf((*common.ErrorInfo)(nil)).Elem()
)

// Forward 消息转发
type Forward struct {
	allModules []interface{}
	protocoles map[uint16]*ProtocolMethod
	// 按玩家串行的分片执行器：同一玩家串行，玩家之间并发
	dispatcher *playerDispatcher
}

// ProtocolMethod 远程调用后, 协议对应的函数
type ProtocolMethod struct {
	// 调用的函数方法, 例如 Role 下的 EnterHandler
	method reflect.Method //处理器

	// 函数入参
	reqTyp reflect.Type
	// 函数出参
	respTyp reflect.Type

	// 调用函数的示例, 例如 Role
	moduleTyp reflect.Type
	moduleVal reflect.Value

	moduleName string
	methodName string
}

// NewForward ..
func NewForward() *Forward {
	return &Forward{
		protocoles: make(map[uint16]*ProtocolMethod),
		dispatcher: newPlayerDispatcher(playerShardCount, playerQueueLen),
	}
}

// Name ..
func (pm *ProtocolMethod) Name() string {
	return pm.moduleName + "." + pm.methodName
}

// AddModules 添加模块
func (f *Forward) AddModules(modules []interface{}) error {
	// 1. 注册模块,按照字母顺序排列
	f.allModules = modules

	// 2. 模块注册
	if err := f.register(); err != nil {
		// 注册失败直接结束
		panic(err)
		return fmt.Errorf("register modules failed: %w", err)
	}

	return nil
}

// register 遍历对外处理函数,与协议绑定
func (f *Forward) register() error {
	var err error
	f.protocoles, err = ParsedProtocolMethods(f.allModules)
	if err != nil {
		return err
	}
	if len(f.protocoles) == 0 {
		return errors.New("no protocol methods registered")
	}

	log2.Get().Info("[game.forward.register] success", zap.Int("num", len(f.protocoles)))
	return nil
}

// Call 调用协议函数
/**
 * p:玩家信息 Player *Player
* req 请求体
* resp 响应
*/
func (pm *ProtocolMethod) Call(ctx context.Context, p any, req *common.RpcMessage, resp *common.Resp) error {
	reqParam := req.Data.Body //protobuf
	// 对象池获取
	protocolReq := bytes.Types().Get(pm.reqTyp).(proto.Message)
	if err := proto.Unmarshal(reqParam, protocolReq); err != nil {
		return fmt.Errorf("failed to unmarshal, request: %s, err: %v", pm.Name(), err)
	}

	// 2. 协议函数返回值
	protocolResp := bytes.Types().Get(pm.respTyp).(proto.Message)

	// 3. 调用协议函数
	results := pm.method.Func.Call([]reflect.Value{
		pm.moduleVal,
		reflect.ValueOf(ctx),
		reflect.ValueOf(p),
		reflect.ValueOf(protocolReq),
		reflect.ValueOf(protocolResp),
	})
	bytes.Types().Put(pm.reqTyp, protocolReq)
	if results != nil {
		if !results[0].IsNil() {
			if errResult := results[0].Interface().(*common.ErrorInfo); errResult != nil {
				resp.Code = errResult.Code
				resp.Flag = errResult.Flag
				common.FreeErrorInfo(errResult)
			}
		}
	}
	// 4.2 协议返回值写入远程调用的返回值中
	respData, err := proto.Marshal(protocolResp)
	bytes.Types().Put(pm.respTyp, protocolResp)
	if err != nil {
		return fmt.Errorf("failed to marshal, request: %s, err: %v", pm.Name(), err)
	}

	resp.Body = respData
	return err
}

// ParsedProtocolMethods 解析协议函数
func ParsedProtocolMethods(rcvrs []interface{}) (map[uint16]*ProtocolMethod, error) {
	// 1. 解析
	repeated := []string{}
	protocoles := make(map[uint16]*ProtocolMethod)
	for _, rcvr := range rcvrs {
		l, err := parsedProtocolMethod(rcvr)
		if err != nil {
			return nil, err
		}

		for protocol, pm := range l {
			if prev, found := protocoles[protocol]; found {
				repeated = append(repeated, fmt.Sprintf("protocol: %d, method: %s, %s", protocol, prev.Name(), pm.Name()))
				continue
			}

			protocoles[protocol] = pm
		}
	}

	if len(repeated) > 0 {
		for _, v := range repeated {
			log2.Get().Error("repeated protocol", zap.String("protocol", v))
		}
		return nil, errors.New("protocol already exists")
	}

	return protocoles, nil
}

// parsedProtocolMethod 解析协议函数
func parsedProtocolMethod(rcvr any) (map[uint16]*ProtocolMethod, error) {
	if rcvr == nil {
		return nil, errors.New("rcvr cannot be nil")
	}

	moduleTyp := reflect.TypeOf(rcvr)
	moduleVal := reflect.ValueOf(rcvr)

	methodNum := moduleTyp.NumMethod()

	out := make(map[uint16]*ProtocolMethod, methodNum)

	moduleName := reflect.Indirect(moduleVal).Type().Name()
	if err := checkModuleName(moduleName); err != nil {
		return nil, err
	}

	for protoId, methodName := range protoHandlerInit.ProtoIdToMethodMap {
		for i := range methodNum {
			method := moduleTyp.Method(i)
			if method.Name != methodName {
				continue
			}
			// 1.必须是指针
			if moduleTyp.Kind() != reflect.Ptr {
				panic("service must be pointer")
			}
			// 2. 函数必须是导出的
			if method.PkgPath != "" {
				continue
			}

			// 4. 函数参数一定只有 4 个: ctx, id(uuid/roleID), req, resp
			methodType := method.Type
			if methodType.NumIn() != 5 {
				return nil, fmt.Errorf("protocol function must have exactly 3 parameters, please check %s.%s", moduleName, methodName)
			}

			// 4.1 第一个参数为 context.Context
			ctxType := methodType.In(1)
			if !ctxType.Implements(typeOfContext) {
				return nil, fmt.Errorf("first parameter of protocol function must be context.Context, please check %s.%s", moduleName, methodName)
			}

			// 4.3 第三个参数
			// req
			reqTyp := methodType.In(3)
			if err := checkReqParam(reqTyp, int32(protoId)); err != nil {
				return nil, fmt.Errorf("second parameter %s, please check %s.%s", err.Error(), moduleName, methodName)
			}

			// 4.3 第四个参数
			// resp
			respTyp := methodType.In(4)
			if err := checkRespParam(respTyp, int32(protoId)); err != nil {
				return nil, fmt.Errorf("third parameter %s, please check %s.%s", err.Error(), moduleName, methodName)
			}

			if methodType.NumOut() == 1 && !methodType.Out(0).AssignableTo(typeOfResult) && !methodType.Out(0).AssignableTo(reflect.PtrTo(typeOfResult)) {
				// 处理错误
				return nil, fmt.Errorf("single return value must be Result type, please check %s.%s", moduleName, methodName)
			}

			out[protoId] = &ProtocolMethod{
				method:     method,
				reqTyp:     reqTyp,
				respTyp:    respTyp,
				moduleTyp:  moduleTyp,
				moduleVal:  moduleVal,
				moduleName: moduleName,
				methodName: methodName,
			}
			bytes.Types().Add(reqTyp)
			bytes.Types().Add(respTyp)
		}
	}
	return out, nil
}

func checkModuleName(moduleName string) error {
	moduleNameLower := strings.ToLower(moduleName)
	if strings.Contains(moduleNameLower, "req") ||
		strings.Contains(moduleNameLower, "resp") {
		return fmt.Errorf("模块名: %s 不可包含 req 或者 resp", moduleName)
	}
	return nil
}

func checkMethodName(methodName string) error {
	methodNameLower := strings.ToLower(methodName)
	if strings.Contains(methodNameLower, "req") ||
		strings.Contains(methodNameLower, "resp") {
		return fmt.Errorf("函数名称: %s 不可包含 req 或者 resp", methodName)
	}
	return nil
}

func checkReqParam(t reflect.Type, protocol int32) error {
	if err := checkInParam(t); err != nil {
		return err
	}
	_ = protocol

	return nil
}

func checkRespParam(t reflect.Type, protocol int32) error {
	if err := checkInParam(t); err != nil {
		return err
	}
	_ = protocol

	return nil
}

func checkInParam(t reflect.Type) error {
	if t.Kind() != reflect.Ptr {
		return errors.New("must be a pointer type")
	}
	// 检查是否为空
	if t.Elem().Kind() == reflect.Invalid {
		return fmt.Errorf("cannot be nil")
	}
	// 必须实现 proto.Message 接口
	if !t.Implements(typeOfProtoMsg) {
		return fmt.Errorf("must implement proto.Message interface")
	}
	return nil
}
