package task

const (
	taskKey     = "task:uid:%d:tid:%d"
	progressKey = "progress:uid:%d:pid:%d"

	taskCollection     = "user_tasks"
	progressCollection = "user_progress"

	// 任务状态
	TaskStatusPending   = 0 // 未开始
	TaskStatusRunning   = 1 // 进行中
	TaskStatusCompleted = 2 // 已完成
	TaskStatusRewarded  = 3 // 已领奖

	// 任务类型
	TaskTypeDaily    = 3 // 每天重置
	TaskTypePerRound = 5 // 每局重置
)

// Task 任务。唯一键 (uid, tId)。
type Task struct {
	Uid            int64         `bson:"uid" json:"uid"`
	TId            int           `bson:"tId" json:"tId"`
	CreatTimestamp int64         `bson:"creatTimestamp" json:"creatTimestamp"` //创建任务的时间戳,秒
	RewardTime     int64         `bson:"rewardTime" json:"rewardTime"`         //领奖时间
	Status         int           `bson:"status" json:"status"`                 //状态
	PMap           map[int]int64 `bson:"pMap" json:"pMap"`                     //进度id—进度开始值
	UpdatedAt      int64         `bson:"updatedAt" json:"updatedAt"`
}

// Progress 进度。唯一键 (uid, pId)。
type Progress struct {
	Uid        int64 `bson:"uid" json:"uid"`
	PId        int   `bson:"pId" json:"pId"`
	CurV       int64 `bson:"curV" json:"curV"`             //当前进度值，完成任务时不需要清0
	UpdateTime int64 `bson:"updateTime" json:"updateTime"` //更新进度的时间戳,秒
}
