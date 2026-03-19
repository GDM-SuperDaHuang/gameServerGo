package room

//
//import (
//	"gameServer/common/errorCode"
//	"sync"
//)
//
//// 局信息
//type round struct {
//	lock       sync.RWMutex //
//	operations []*Operation //userid -本局玩家的操作
//	index      uint8        //第几局,0开始
//	timeOut    int64        //超时时间戳
//	creatTime  int64        //创建时间戳 秒
//}
//
//type roundSafety struct {
//	lock      sync.RWMutex //
//	roundList []*round
//}
//
//func (r *roundSafety) len() int {
//	r.lock.RLock()
//	defer r.lock.RUnlock()
//	return len(r.roundList)
//}
//
//func (r *round) addOp(op *Operation) {
//	for _, info := range r.operations {
//		if info.userId == op.userId {
//			info.consume = op.consume
//			info.operation = op.operation
//			return
//		}
//	}
//}
//
//func (r *round) isReusingBet(userId uint64) bool {
//	r.lock.RLock()
//	defer r.lock.RUnlock()
//	for _, info := range r.operations {
//		if info.userId == userId {
//			if len(info.consume) > 0 {
//				return true
//			}
//		}
//	}
//	return false
//}
//
//func (r *roundSafety) addRound(nowRound *round) {
//	r.lock.Lock()
//	defer r.lock.Unlock()
//	r.roundList = append(r.roundList, nowRound)
//}
//
//// todo 安全问题
//func (r *roundSafety) getCurRound() *round {
//	r.lock.RLock()
//	defer r.lock.RUnlock()
//	return r.roundList[len(r.roundList)-1]
//}
//
//func (r *roundSafety) isReusing(userId uint64, itemId int) bool {
//	r.lock.RLock()
//	defer r.lock.RUnlock()
//	for _, ls := range r.roundList {
//		for _, info := range ls.operations {
//			if info.userId == userId {
//				for k, _ := range info.consume {
//					if k == itemId {
//						return true
//					}
//				}
//			}
//		}
//
//	}
//	return false
//}
//
//// todo 安全问题
//func (r *roundSafety) getCurRoundOp(userId uint64) uint16 {
//	r.lock.RLock()
//	defer r.lock.RUnlock()
//	curRound := r.roundList[len(r.roundList)-1]
//	for _, info := range curRound.operations {
//		if info.userId == userId && info.operation == opBet {
//			return errorCode.ErrorCode_AlreadyBet
//		} else if info.userId == userId && info.operation == opAbstain {
//			return errorCode.ErrorCode_AlreadyAbstain
//		} else if info.userId == userId {
//			return 0
//		}
//	}
//	return 0
//}
//
//func (r *roundSafety) getRound(index int) *round {
//	r.lock.RLock()
//	defer r.lock.RUnlock()
//	return r.roundList[index]
//}
//
//// 获取上一局
//func (r *roundSafety) getLastRound() *round {
//	r.lock.RLock()
//	defer r.lock.RUnlock()
//	if len(r.roundList) < 2 {
//		return nil
//	}
//	return r.roundList[len(r.roundList)-2]
//}
