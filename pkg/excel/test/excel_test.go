package test

import (
	"fmt"
	"gameServer/pkg/excel/models"
	"gameServer/pkg/excel/reader"
	"log"
	"testing"
)

func TestRuntt(t *testing.T) {
	// 创建读取器（指向excels目录）
	r := reader.NewExcelReader("./excels")

	// ========== 方式1：读取所有Excel的所有Sheet（原始数据） ==========
	fmt.Println("=== 方式1：读取所有Excel原始数据 ===")
	allData, err := r.ReadAllExcels()
	if err != nil {
		log.Fatal(err)
	}

	for fileName, sheets := range allData {
		fmt.Printf("\n文件: %s\n", fileName)
		for sheetName, rows := range sheets {
			fmt.Printf("  Sheet: %s, 行数: %d\n", sheetName, len(rows))
			if len(rows) > 0 {
				fmt.Printf("  表头: %v\n", rows[0])
			}
		}
	}

	// ========== 方式2：读取指定文件指定Sheet到结构体 ==========
	fmt.Println("\n=== 方式2：映射到结构体 ===")
	//var products = []models.Product{}
	var products = []*models.Product{}
	allStructMap := map[string]interface{}{
		"product": &products,
	}
	err = r.ReadSheetToStruct(allData, allStructMap)
	if err != nil {
		log.Printf("读取失败: %v", err)
		// 尝试读取其他sheet或创建示例数据演示
	} else {
		for i, p := range products {
			fmt.Printf("产品%d: %+v\n", i+1, p)
		}
	}

	// ========== 方式3：链式调用（推荐） ==========
	//fmt.Println("\n=== 方式3：链式API调用 ===")
	//loader := reader.NewLoader("./excels")
	//
	//var users []models.user
	//err = loader.Load("users.xlsx").Sheet("用户信息").Into(&users)
	//if err != nil {
	//	log.Printf("读取用户失败: %v", err)
	//} else {
	//	for _, u := range users {
	//		fmt.Printf("用户: RoomType=%d, 姓名=%s, 年龄=%d, 邮箱=%v, 分数=%v, VIP=%v\n",
	//			u.UserID, u.UserName, u.Age, u.Emails, u.Scores, u.VIP)
	//	}
	//}

	// ========== 方式4：自定义类型解析器 ==========
	//fmt.Println("\n=== 方式4：自定义解析 ===")
	//parser := reader.NewTypeParser()
	//
	//// 注册自定义类型解析（示例：解析时间）
	//parser.RegisterCustomParser(reflect.TypeOf(time.Time{}), func(s string) (interface{}, error) {
	//	return time.Parse("2006-01-02", s)
	//})
	//
	//// 使用自定义parser创建reader
	//customReader := reader.NewExcelReader("./excels")
	//_ = customReader
	//
	//// ========== 演示数据结构解析 ==========
	//fmt.Println("\n=== 数据结构解析演示 ===")
	//demoParser := reader.NewTypeParser()
	//
	//// 解析字符串切片
	//tags, _ := demoParser.Parse("go,python,java", reflect.TypeOf([]string{}))
	//fmt.Printf("Tags: %v (类型: %T)\n", tags, tags)
	//
	//// 解析Map
	//attrs, _ := demoParser.Parse("1:100; 2:200; 3:300", reflect.TypeOf(map[int]int{}))
	//fmt.Printf("Attributes: %v (类型: %T)\n", attrs, attrs)
}
