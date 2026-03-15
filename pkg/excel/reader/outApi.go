package reader

import "fmt"

// ExcelLoader 便捷加载器
type ExcelLoader struct {
	reader *ExcelReader
}

// NewLoader 创建加载器
func NewLoader(basePath string) *ExcelLoader {
	return &ExcelLoader{
		reader: NewExcelReader(basePath),
	}
}

// Load 链式调用入口
func (l *ExcelLoader) Load(fileName string) *SheetSelector {
	return &SheetSelector{
		loader:   l,
		fileName: fileName,
		reader:   l.reader,
	}
}

// SheetSelector Sheet选择器
type SheetSelector struct {
	loader    *ExcelLoader
	fileName  string
	sheetName string
	reader    *ExcelReader
}

// Sheet 选择Sheet
func (s *SheetSelector) Sheet(sheetName string) *SheetSelector {
	s.sheetName = sheetName
	return s
}

// Into 映射到结构体切片
//func (s *SheetSelector) Into(slicePtr interface{}) error {
//	if s.sheetName == "" {
//		return fmt.Errorf("必须先调用Sheet()选择工作表")
//	}
//	return s.reader.ReadSheetToStruct(s.fileName, s.sheetName, slicePtr)
//}

// GetRaw 获取原始数据
func (s *SheetSelector) GetRaw() ([][]string, error) {
	sheets, err := s.reader.ReadExcel(s.fileName)
	if err != nil {
		return nil, err
	}

	if s.sheetName == "" {
		// 返回第一个Sheet
		for _, rows := range sheets {
			return rows, nil
		}
		return nil, fmt.Errorf("文件为空")
	}

	rows, ok := sheets[s.sheetName]
	if !ok {
		return nil, fmt.Errorf("Sheet '%s' 不存在", s.sheetName)
	}
	return rows, nil
}
