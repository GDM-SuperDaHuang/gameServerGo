package models

// Product 产品模型
type Product struct {
	ID         int64       `excel:"RoomType"`
	Name       string      `excel:"name"`
	Price      float64     `excel:"price"`
	Tags       []int       `excel:"tags"`       // 自动解析逗号分隔: tag1,tag2,tag3
	Attributes map[int]int `excel:"Attributes"` // 自动解析: 1:100; 2:200; 3:300
	IsActive   bool        `excel:"IsActive"`   // 注意：标签要去掉空格
}

// User 用户模型示例
type User struct {
	UserID   int64       `excel:"userID"`
	UserName string      `excel:"userName"`
	Age      int         `excel:"age"`
	Emails   []string    `excel:"emails"` // 多个邮箱逗号分隔
	Scores   map[int]int `excel:"scores"` // 科目:分数;数学:90.5;语文:88.0
	VIP      bool        `excel:"vip"`
}

// Order 订单模型
type Order struct {
	OrderID     string        `excel:"订单号"`
	ProductIDs  []int64       `excel:"商品ID列表"` // 1,2,3,4
	Quantities  map[int64]int `excel:"数量映射"`   // 1001:2; 1002:5
	TotalAmount float64       `excel:"总金额"`
	Paid        bool          `excel:"已支付"`
}
