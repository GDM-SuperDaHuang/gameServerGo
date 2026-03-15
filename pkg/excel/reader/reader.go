package reader

import (
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/xuri/excelize/v2"
)

// ExcelReader Excel读取器封装
type ExcelReader struct {
	basePath string
	parser   *TypeParser
}

// NewExcelReader 创建新的Excel读取器
func NewExcelReader(basePath string) *ExcelReader {
	return &ExcelReader{
		basePath: basePath,
		parser:   NewTypeParser(),
	}
}

// ReadAllExcels 读取目录下所有Excel文件的所有Sheet
func (r *ExcelReader) ReadAllExcels() (map[string]map[string][][]string, error) {
	result := make(map[string]map[string][][]string)

	// 查找所有xlsx文件
	pattern := filepath.Join(r.basePath, "*.xlsx")

	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("查找Excel文件失败: %w", err)
	}

	for _, file := range files {
		fileName := filepath.Base(file)
		if strings.Contains(fileName, "~") { //忽视打开的文件
			continue
		}
		sheets, err := r.ReadExcel(fileName)
		if err != nil {
			return nil, fmt.Errorf("读取文件 %s 失败: %w", fileName, err)
		}
		result[fileName] = sheets
	}

	return result, nil
}

// ReadExcel 读取指定Excel文件的所有Sheet
func (r *ExcelReader) ReadExcel(fileName string) (map[string][][]string, error) {
	filePath := filepath.Join(r.basePath, fileName)
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("打开文件失败: %w", err)
	}
	defer f.Close()

	result := make(map[string][][]string)
	sheetList := f.GetSheetList()

	for _, sheetName := range sheetList {
		rows, err := f.GetRows(sheetName)
		if err != nil {
			return nil, fmt.Errorf("读取Sheet %s 失败: %w", sheetName, err)
		}
		result[sheetName] = rows
	}

	return result, nil
}

func (r *ExcelReader) findType(fileType string) {
	switch fileType {
	case "int":
	case "string":
	case "list<int>":
	case "bool":
	case "map<int,int>":
	case "map<int,string>":
	case "time":
	default:
	}
}

// ReadSheetToStruct 读取指定Sheet并映射到结构体切片
// allData map[string]map[string][][]string --excelName：sheetName：data
func (r *ExcelReader) ReadSheetToStruct(allData map[string]map[string][][]string, slicePtrMap map[string]interface{}) error {
	// 验证slicePtr是指向切片的指针
	for sheetName, structs := range slicePtrMap {
		sliceValue := reflect.ValueOf(structs)
		if sliceValue.Kind() != reflect.Ptr || sliceValue.Elem().Kind() != reflect.Slice {
			return fmt.Errorf("slicePtr必须是指向切片的指针")
		}
		//获取元素类型
		elemType := sliceValue.Elem().Type().Elem()
		if elemType.Kind() == reflect.Ptr {
			elemType = elemType.Elem()
		}

		for excelName, info := range allData {
			_, ok := info[sheetName]
			if !ok {
				panic(errors.New(fmt.Sprintf("不存在结构 %s", sheetName)))
			}

			// 读取Excel数据
			filePath := filepath.Join(r.basePath, excelName)
			f, err := excelize.OpenFile(filePath)
			if err != nil {
				return fmt.Errorf("打开文件失败: %w", err)
			}

			defer f.Close()

			rows, err := f.GetRows(sheetName)
			if err != nil {
				return fmt.Errorf("读取Sheet失败: %w", err)
			}

			if len(rows) < 2 {
				return fmt.Errorf("Sheet数据不足，至少需要表头和一行数据")
			}

			// 解析表头
			headers := rows[0]
			fieldMap := buildFieldMap(elemType)

			// 解析数据行
			dataRows := rows[1:]
			resultSlice := reflect.MakeSlice(sliceValue.Elem().Type(), 0, len(dataRows))

			for rowIdx, row := range dataRows {
				elem := reflect.New(elemType).Elem()

				for colIdx, cellValue := range row {
					if colIdx >= len(headers) {
						continue
					}

					header := strings.TrimSpace(headers[colIdx])
					if fieldInfo, ok := fieldMap[header]; ok {
						if err := r.setFieldValue(elem, fieldInfo, cellValue); err != nil {
							return fmt.Errorf("第%d行,列'%s': %w", rowIdx+2, header, err)
						}
					}
				}
				// 报错
				resultSlice = reflect.Append(resultSlice, elem.Addr())
			}
			sliceValue.Elem().Set(resultSlice)

		}
	}

	return nil
}

//func (r *ExcelReader) ReadSheetToStruct1(allData map[string]map[string][][]string, slicePtrMap map[string]interface{}) error {
//	// 验证slicePtr是指向切片的指针
//	for sheetName, value := range slicePtrMap {
//		sliceValue := reflect.ValueOf(value)
//		//if sliceValue.Kind() != reflect.Ptr || sliceValue.Elem().Kind() != reflect.Slice {
//		//	return fmt.Errorf("slicePtr必须是指向切片的指针")
//		//}
//		// 获取元素类型
//		elemType := sliceValue.Elem().Type().Elem()
//		if elemType.Kind() == reflect.Ptr {
//			elemType = elemType.Elem()
//		}
//
//		for excelName, info := range allData {
//			_, ok := info[sheetName]
//			if !ok {
//				panic(errors.New(fmt.Sprintf("不存在结构 %s", sheetName)))
//			}
//
//			// 读取Excel数据
//			filePath := filepath.Join(r.basePath, excelName)
//			f, err := excelize.OpenFile(filePath)
//			if err != nil {
//				return fmt.Errorf("打开文件失败: %w", err)
//			}
//
//			defer f.Close()
//
//			rows, err := f.GetRows(sheetName)
//			if err != nil {
//				return fmt.Errorf("读取Sheet失败: %w", err)
//			}
//
//			if len(rows) < 2 {
//				return fmt.Errorf("Sheet数据不足，至少需要表头和一行数据")
//			}
//
//			// 解析表头
//			headers := rows[0]
//			fieldMap := buildFieldMap(elemType)
//
//			// 解析数据行
//			dataRows := rows[1:]
//			resultSlice := reflect.MakeSlice(sliceValue.Elem().Type(), 0, len(dataRows))
//
//			for rowIdx, row := range dataRows {
//				elem := reflect.New(elemType).Elem()
//
//				for colIdx, cellValue := range row {
//					if colIdx >= len(headers) {
//						continue
//					}
//
//					header := strings.TrimSpace(headers[colIdx])
//					if fieldInfo, ok := fieldMap[header]; ok {
//						if err := r.setFieldValue(elem, fieldInfo, cellValue); err != nil {
//							return fmt.Errorf("第%d行,列'%s': %w", rowIdx+2, header, err)
//						}
//					}
//				}
//
//				resultSlice = reflect.Append(resultSlice, elem.Addr())
//			}
//
//			sliceValue.Elem().Set(resultSlice)
//
//		}
//	}
//
//	return nil
//
//}

// fieldInfo 字段信息
type fieldInfo struct {
	Index     int
	Type      reflect.Type
	FieldName string
}

// buildFieldMap 构建Excel标签到字段的映射
func buildFieldMap(t reflect.Type) map[string]fieldInfo {
	fieldMap := make(map[string]fieldInfo)

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tag := field.Tag.Get("excel")
		if tag == "" {
			continue
		}

		// 处理标签，去除空格
		tag = strings.TrimSpace(tag)
		fieldMap[tag] = fieldInfo{
			Index:     i,
			Type:      field.Type,
			FieldName: field.Name,
		}
	}

	return fieldMap
}

// setFieldValue 设置字段值
func (r *ExcelReader) setFieldValue(elem reflect.Value, info fieldInfo, value string) error {
	field := elem.Field(info.Index)
	value = strings.TrimSpace(value)

	parsedValue, err := r.parser.Parse(value, info.Type)
	if err != nil {
		return fmt.Errorf("解析'%s'为%v失败: %w", value, info.Type, err)
	}

	field.Set(reflect.ValueOf(parsedValue))
	return nil
}
