package reader

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// TypeParser 类型解析器
type TypeParser struct {
	customParsers map[reflect.Type]func(string) (interface{}, error)
}

// NewTypeParser 创建类型解析器
func NewTypeParser() *TypeParser {
	parser := &TypeParser{
		customParsers: make(map[reflect.Type]func(string) (interface{}, error)),
	}
	parser.registerDefaultParsers()
	return parser
}

// RegisterCustomParser 注册自定义类型解析器
func (p *TypeParser) RegisterCustomParser(t reflect.Type, parser func(string) (interface{}, error)) {
	p.customParsers[t] = parser
}

// Parse 解析字符串为目标类型
func (p *TypeParser) Parse(value string, targetType reflect.Type) (interface{}, error) {
	// 检查自定义解析器
	if parser, ok := p.customParsers[targetType]; ok {
		return parser(value)
	}

	// 基础类型解析
	switch targetType.Kind() {
	case reflect.String:
		return value, nil

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if value == "" {
			return reflect.Zero(targetType).Interface(), nil
		}
		v, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return nil, err
		}
		return reflect.ValueOf(v).Convert(targetType).Interface(), nil

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if value == "" {
			return reflect.Zero(targetType).Interface(), nil
		}
		v, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return nil, err
		}
		return reflect.ValueOf(v).Convert(targetType).Interface(), nil

	case reflect.Float32, reflect.Float64:
		if value == "" {
			return reflect.Zero(targetType).Interface(), nil
		}
		v, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nil, err
		}
		return reflect.ValueOf(v).Convert(targetType).Interface(), nil

	case reflect.Bool:
		if value == "" {
			return false, nil
		}
		v, err := strconv.ParseBool(value)
		if err != nil {
			// 支持中文和自定义bool值
			lower := strings.ToLower(value)
			if lower == "是" || lower == "yes" || lower == "1" || lower == "true" {
				return true, nil
			}
			if lower == "否" || lower == "no" || lower == "0" || lower == "false" {
				return false, nil
			}
			return nil, err
		}
		return v, nil

	case reflect.Slice:
		return p.parseSlice(value, targetType)
	case reflect.Map:
		return p.parseMap(value, targetType)
	default:
		return nil, fmt.Errorf("不支持的类型: %v", targetType)
	}
}

// parseSlice 解析切片类型
func (p *TypeParser) parseSlice(value string, targetType reflect.Type) (interface{}, error) {
	elemType := targetType.Elem()

	// 字符串切片 - 逗号分隔
	if elemType.Kind() == reflect.String {
		if value == "" {
			return reflect.MakeSlice(targetType, 0, 0).Interface(), nil
		}

		parts := strings.Split(value, ",")
		result := reflect.MakeSlice(targetType, 0, len(parts))

		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part != "" {
				result = reflect.Append(result, reflect.ValueOf(part))
			}
		}
		return result.Interface(), nil
	}

	// 整数切片
	//if elemType.Kind() >= reflect.Int && elemType.Kind() <= reflect.Int64 {
	if elemType.Kind() >= reflect.Bool && elemType.Kind() <= reflect.Uint64 {
		if value == "" {
			return reflect.MakeSlice(targetType, 0, 0).Interface(), nil
		}

		parts := strings.Split(value, ",")
		result := reflect.MakeSlice(targetType, 0, len(parts))

		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			v, err := strconv.ParseInt(part, 10, 64)
			if err != nil {
				return nil, err
			}
			result = reflect.Append(result, reflect.ValueOf(v).Convert(elemType))
		}
		return result.Interface(), nil
	}

	return nil, fmt.Errorf("不支持的切片元素类型: %v", elemType)
}

// parseMap 解析Map类型
// 现在支持格式: key1:value1; key2:value2; key3:value3
// 需要增加支持格式2: key1:value1,value2,value3; key2:value4,value5; key3:value3
//func (p *TypeParser) parseMap(value string, targetType reflect.Type) (interface{}, error) {
//	keyType := targetType.Key()
//	elemType := targetType.Elem()
//
//	result := reflect.MakeMap(targetType)
//
//	if value == "" {
//		return result.Interface(), nil
//	}
//
//	// 现在支持格式: key1:value1; key2:value2; key3:value3
//	pairs := strings.Split(value, ";")
//
//	for _, pair := range pairs {
//		pair = strings.TrimSpace(pair)
//		if pair == "" {
//			continue
//		}
//
//		kv := strings.SplitN(pair, ":", 2)
//		if len(kv) != 2 {
//			continue // 跳过格式错误的
//		}
//
//		keyStr := strings.TrimSpace(kv[0])
//		valStr := strings.TrimSpace(kv[1])
//
//		// 解析key
//		var key reflect.Value
//		switch keyType.Kind() {
//		case reflect.String:
//			key = reflect.ValueOf(keyStr)
//		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
//			k, err := strconv.ParseInt(keyStr, 10, 64)
//			if err != nil {
//				return nil, fmt.Errorf("map key解析失败: %s", keyStr)
//			}
//			key = reflect.ValueOf(k).Convert(keyType)
//		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
//			k, err := strconv.ParseUint(keyStr, 10, 64)
//			if err != nil {
//				return nil, fmt.Errorf("map key解析失败: %s", keyStr)
//			}
//			key = reflect.ValueOf(k).Convert(keyType)
//		default:
//			return nil, fmt.Errorf("不支持的map key类型: %v", keyType)
//		}
//
//		// 解析value
//		var val reflect.Value
//		switch elemType.Kind() {
//		case reflect.String:
//			val = reflect.ValueOf(valStr)
//		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
//			v, err := strconv.ParseInt(valStr, 10, 64)
//			if err != nil {
//				return nil, fmt.Errorf("map value解析失败: %s", valStr)
//			}
//			val = reflect.ValueOf(v).Convert(elemType)
//		case reflect.Float32, reflect.Float64:
//			v, err := strconv.ParseFloat(valStr, 64)
//			if err != nil {
//				return nil, fmt.Errorf("map value解析失败: %s", valStr)
//			}
//			val = reflect.ValueOf(v).Convert(elemType)
//		default:
//			return nil, fmt.Errorf("不支持的map value类型: %v", elemType)
//		}
//
//		result.SetMapIndex(key, val)
//	}
//
//	return result.Interface(), nil
//}

func (p *TypeParser) parseMap(value string, targetType reflect.Type) (interface{}, error) {
	keyType := targetType.Key()
	elemType := targetType.Elem()

	result := reflect.MakeMap(targetType)

	if value == "" {
		return result.Interface(), nil
	}

	// key:value;key:value
	pairs := strings.Split(value, ";")

	for _, pair := range pairs {

		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}

		kv := strings.SplitN(pair, ":", 2)
		if len(kv) != 2 {
			continue
		}

		keyStr := strings.TrimSpace(kv[0])
		valStr := strings.TrimSpace(kv[1])

		// 解析key
		keyParsed, err := p.Parse(keyStr, keyType)
		if err != nil {
			return nil, fmt.Errorf("map key解析失败: %w", err)
		}

		// 解析value（关键：递归）
		valParsed, err := p.Parse(valStr, elemType)
		if err != nil {
			return nil, fmt.Errorf("map value解析失败: %w", err)
		}

		result.SetMapIndex(
			reflect.ValueOf(keyParsed).Convert(keyType),
			reflect.ValueOf(valParsed).Convert(elemType),
		)
	}

	return result.Interface(), nil
}

// registerDefaultParsers 注册默认解析器
func (p *TypeParser) registerDefaultParsers() {
	// 可以在这里添加更多自定义类型解析
}
