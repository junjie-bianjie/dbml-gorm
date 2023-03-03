package gen

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/dave/jennifer/jen"
	"github.com/duythinht/dbml-go/core"
	"github.com/duythinht/dbml-go/internal/gen-go-model/genutil"
)

type generator struct {
	dbml             *core.DBML
	out              string
	gopackage        string
	fieldtags        []string
	types            map[string]jen.Code
	shouldGenTblName bool
}

func newgen() *generator {
	return &generator{
		types: make(map[string]jen.Code),
	}
}

func (g *generator) reset(rememberAlias bool) {
	g.dbml = nil
	if !rememberAlias {
		g.types = make(map[string]jen.Code)
	}
}

func (g *generator) file() *jen.File {
	return jen.NewFilePathName(g.out, g.gopackage)
}

func (g *generator) generate() error {
	if err := g.genEnums(); err != nil {
		return err
	}
	return nil
}

func (g *generator) genEnums() error {
	for _, enum := range g.dbml.Enums {
		if err := g.genEnum(enum); err != nil {
			return err
		}
	}
	for _, table := range g.dbml.Tables {
		if err := g.genTable(table); err != nil {
			return err
		}
	}

	return nil
}

func (g *generator) genEnum(enum core.Enum) error {
	f := jen.NewFilePathName(g.out, g.gopackage)

	enumOriginName := genutil.NormalizeTypeName(enum.Name)
	enumGoTypeName := genutil.NormalizeGoTypeName(enum.Name)

	f.Commentf("%s is generated type for enum '%s'", enumGoTypeName, enumOriginName)
	f.Type().Id(enumGoTypeName).Int()

	f.Const().DefsFunc(func(group *jen.Group) {
		group.Id("_").Id(enumGoTypeName).Op("=").Iota()
		for _, value := range enum.Values {
			v := group.Id(genutil.NormalLizeGoName(value.Name))
			if value.Note != "" {
				v.Comment(value.Note)
			}
		}
	})

	g.types[enum.Name] = jen.Id(enumGoTypeName)

	return f.Save(fmt.Sprintf("%s/%s.enum.go", g.out, genutil.Normalize(enum.Name)))
}

func (g *generator) genTable(table core.Table) error {
	f := jen.NewFilePathName(g.out, g.gopackage)

	tableOriginName := genutil.Normalize(table.Name)
	tableGoTypeName := genutil.NormalizeGoTypeName(table.Name)

	f.PackageComment("Code generated by dbml-gen-gorm. DO NOT EDIT.")

	var genColumnErr error

	cols := make([]string, 0)

	f.Type().Id(tableGoTypeName).StructFunc(func(group *jen.Group) {
		for _, column := range table.Columns {
			columnName := genutil.NormalLizeGoName(column.Name)
			columnOriginName := genutil.Normalize(column.Name)
			t, ok := g.getJenType(column.Type)
			if !ok {
				genColumnErr = fmt.Errorf("type '%s' is not support", column.Type)
			}
			if column.Settings.Note != "" {
				group.Comment(column.Settings.Note)
			}

			gotags := make(map[string]string)
			for _, t := range g.fieldtags {
				tagName := strings.TrimSpace(t)
				if tagName == "gorm" {
					gotags[tagName] = "column:" + columnOriginName
				} else {
					gotags[tagName] = columnOriginName
				}
			}
			group.Id(columnName).Add(t).Tag(gotags)
			cols = append(cols, columnOriginName)
		}
	})

	if genColumnErr != nil {
		return genColumnErr
	}

	tableMetadataType := genutil.FirstLetterLower(tableGoTypeName) + "TableColumns"
	tableMetadataColumnsType := tableGoTypeName + "TableColumns"

	f.Commentf("// table '%s' columns list struct", tableOriginName)

	f.Type().Id(tableMetadataType).StructFunc(func(group *jen.Group) {
		for _, column := range table.Columns {
			temp := genutil.NormalLizeGoName(column.Name)
			group.Id(temp).String()
		}
	})

	f.Commentf("// table '%s' columns list info", tableOriginName)
	f.Var().Id(tableMetadataColumnsType).Op("=").Id(tableMetadataType).Values(jen.DictFunc(func(d jen.Dict) {
		for _, column := range table.Columns {
			columnName := genutil.NormalLizeGoName(column.Name)
			columnOriginName := tableOriginName + "." + genutil.Normalize(column.Name)
			d[jen.Id(columnName)] = jen.Lit(columnOriginName)
		}
	}))

	f.Commentf("AllColumns return list columns name for table '%s'", tableOriginName)
	f.Func().Params(
		jen.Op("*").Id(tableGoTypeName),
	).Id("AllColumns").Params().Index().String().Block(
		jen.Return(jen.Index().String().ValuesFunc(func(g *jen.Group) {
			for _, col := range cols {
				g.Lit(col)
			}
		})),
	)

	f.Commentf("TableName return table name")
	f.Func().Params(
		jen.Op("*").Id(tableGoTypeName),
	).Id("TableName").Params().Id("string").Block(
		jen.Return(jen.Lit(tableOriginName)),
	)

	tableGoTypeSlice := tableGoTypeName + "Slice"
	f.Type().Id(tableGoTypeSlice).Op(" []").Id(tableGoTypeName)

	return f.Save(fmt.Sprintf("%s/%s.go", g.out, genutil.Normalize(table.Name)))
}

const primeTypePattern = `^(\w+)(\(d+\))?`

var (
	regexType    = regexp.MustCompile(primeTypePattern)
	builtinTypes = map[string]jen.Code{
		"int":        jen.Int(),
		"int8":       jen.Int8(),
		"int16":      jen.Int16(),
		"int32":      jen.Int32(),
		"smallint":   jen.Int16(),
		"tinyint":    jen.Int(),
		"tinyint(1)": jen.Bool(),
		"longtext":   jen.String(),
		"json":       jen.String(),
		"int64":      jen.Int64(),
		"bigint":     jen.Int64(),
		"uint":       jen.Uint(),
		"uint8":      jen.Uint8(),
		"uint16":     jen.Uint16(),
		"uint32":     jen.Uint32(),
		"uint64":     jen.Uint64(),
		"float":      jen.Float64(),
		"float32":    jen.Float32(),
		"float64":    jen.Float64(),
		"bool":       jen.Bool(),
		"text":       jen.String(),
		"varchar":    jen.String(),
		"char":       jen.String(),
		"byte":       jen.Byte(),
		"rune":       jen.Rune(),
		"timestamp":  jen.Int(),
		"datetime":   jen.Qual("time", "Time"),
	}
)

func (g *generator) getJenType(s string) (jen.Code, bool) {
	m := regexType.FindStringSubmatch(s)
	if len(m) >= 2 {
		// lookup for builtin type
		if t, ok := builtinTypes[m[1]]; ok {
			return t, ok
		}
	}
	t, ok := g.types[s]
	return t, ok
}
