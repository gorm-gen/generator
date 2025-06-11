package generator

import (
	"fmt"
	"strings"

	_ "github.com/shopspring/decimal"
	_ "gorm.io/plugin/soft_delete"

	"gorm.io/gen"
	"gorm.io/gorm"
)

type Generator struct {
	db             *gorm.DB
	outPath        string
	mode           gen.GenerateMode
	modelPkgPath   string
	dataTypeMap    map[string]func(gorm.ColumnType) string
	jsonTagName    map[string]JsonTag
	generateModel  []generateModel
	applyBasic     []interface{}
	generator      *gen.Generator
	applyInterface []applyInterface
}

type Option func(*Generator)

func WithOutPath(outPath string) Option {
	return func(g *Generator) {
		g.outPath = outPath
	}
}

func WithMode(mode gen.GenerateMode) Option {
	return func(g *Generator) {
		g.mode = mode
	}
}

func WithModelPkgPath(modelPkgPath string) Option {
	return func(g *Generator) {
		g.modelPkgPath = modelPkgPath
	}
}

type applyInterface struct {
	fc     interface{}
	models []interface{}
}

func (g *Generator) SetApplyInterface(fc interface{}, models ...interface{}) {
	g.applyInterface = append(g.applyInterface, applyInterface{
		fc:     fc,
		models: models,
	})
}

type generateModel struct {
	tableName string
	opts      []gen.ModelOpt
}

func (g *Generator) SetGenerateModel(tableName string, opts ...gen.ModelOpt) {
	gm := generateModel{
		tableName: tableName,
		opts:      make([]gen.ModelOpt, 0),
	}
	if len(opts) > 0 {
		gm.opts = opts
	}
	g.generateModel = append(g.generateModel, gm)
}

func (g *Generator) SetApplyBasic(models ...interface{}) {
	_models := make([]interface{}, 0, len(models))
	for _, model := range models {
		if model == nil {
			continue
		}
		_models = append(_models, model)
	}
	g.applyBasic = append(g.applyBasic, _models...)
}

func WithDataTypeMap(dataTypeMap map[string]func(gorm.ColumnType) string) Option {
	return func(g *Generator) {
		g.dataTypeMap = dataTypeMap
	}
}

func WithReplaceDataTypeMap(dataTypeMap map[string]func(gorm.ColumnType) string) Option {
	return func(g *Generator) {
		if dataTypeMap == nil {
			g.dataTypeMap = nil
			return
		}
		if g.dataTypeMap == nil {
			g.dataTypeMap = make(map[string]func(gorm.ColumnType) string)
		}
		for k, v := range dataTypeMap {
			if v == nil {
				delete(g.dataTypeMap, k)
				continue
			}
			g.dataTypeMap[k] = v
		}
	}
}

func WithJsonTagName(jsonTagName map[string]JsonTag) Option {
	return func(g *Generator) {
		g.jsonTagName = jsonTagName
	}
}

func WithReplaceJsonTagName(jsonTagName map[string]JsonTag) Option {
	return func(g *Generator) {
		if jsonTagName == nil {
			g.jsonTagName = nil
			return
		}
		if g.jsonTagName == nil {
			g.jsonTagName = make(map[string]JsonTag)
		}
		for k, v := range jsonTagName {
			g.jsonTagName[k] = v
		}
	}
}

func New(db *gorm.DB, opts ...Option) *Generator {
	g := &Generator{
		db:             db,
		outPath:        "./internal/query",
		mode:           gen.WithoutContext | gen.WithDefaultQuery | gen.WithQueryInterface,
		modelPkgPath:   "models",
		dataTypeMap:    dataTypeMap(),
		jsonTagName:    jsonTagName(),
		generateModel:  make([]generateModel, 0),
		applyBasic:     make([]interface{}, 0),
		applyInterface: make([]applyInterface, 0),
	}

	for _, opt := range opts {
		opt(g)
	}

	g.generator = gen.NewGenerator(gen.Config{
		OutPath:      g.outPath,
		Mode:         g.mode,
		ModelPkgPath: g.modelPkgPath,
	})

	if g.dataTypeMap != nil && len(g.dataTypeMap) > 0 {
		g.generator.WithDataTypeMap(g.dataTypeMap)
	}

	if g.jsonTagName != nil && len(g.jsonTagName) > 0 {
		g.generator.WithJSONTagNameStrategy(func(columnName string) string {
			if tag, ok := g.jsonTagName[columnName]; ok {
				if tag.Replace != "" {
					columnName = tag.Replace
				}
				var appends string
				for _, v := range tag.Append {
					appends += "," + v
				}
				var adds string
				for _, m := range tag.Add {
					for k, v := range m {
						adds += fmt.Sprintf(`" %s:"%s`, k, v)
					}
				}
				return fmt.Sprintf(`%s%s%s`, columnName, appends, adds)
			}
			return columnName
		})
	}

	g.generator.UseDB(db)

	return g
}

func (g *Generator) Generator() *gen.Generator {
	return g.generator
}

func (g *Generator) Execute() {
	for _, gm := range g.generateModel {
		g.generator.GenerateModel(gm.tableName, gm.opts...)
	}

	if len(g.applyBasic) > 0 {
		g.generator.ApplyBasic(g.applyBasic...)
	}

	if len(g.applyInterface) > 0 {
		for _, ai := range g.applyInterface {
			g.generator.ApplyInterface(ai.fc, ai.models...)
		}
	}

	g.generator.Execute()
}

func dataTypeMap() map[string]func(gorm.ColumnType) string {
	return map[string]func(gorm.ColumnType) string{
		"decimal": func(columnType gorm.ColumnType) (dataType string) {
			return "decimal.Decimal"
		},

		"datetime": func(columnType gorm.ColumnType) (dataType string) {
			if cn := columnType.Name(); cn == "deleted_at" || cn == "deletedAt" || cn == "DeletedAt" {
				return "gorm.DeletedAt"
			}
			if nullable, ok := columnType.Nullable(); nullable && ok {
				return "*time.Time"
			}
			return "time.Time"
		},

		"timestamp": func(columnType gorm.ColumnType) (dataType string) {
			if cn := columnType.Name(); cn == "deleted_at" || cn == "deletedAt" || cn == "DeletedAt" {
				return "gorm.DeletedAt"
			}
			if nullable, ok := columnType.Nullable(); nullable && ok {
				return "*time.Time"
			}
			return "time.Time"
		},

		"tinyint": func(columnType gorm.ColumnType) (dataType string) {
			ct, _ := columnType.ColumnType()
			if strings.HasPrefix(ct, "tinyint(1)") {
				return "bool"
			}
			if nullable, ok := columnType.Nullable(); nullable && ok {
				return "*int8"
			}
			return "int8"
		},

		"smallint": func(columnType gorm.ColumnType) (dataType string) {
			if nullable, ok := columnType.Nullable(); nullable && ok {
				return "*int16"
			}
			return "int16"
		},

		"mediumint": func(columnType gorm.ColumnType) (dataType string) {
			if nullable, ok := columnType.Nullable(); nullable && ok {
				return "*int32"
			}
			return "int32"
		},

		"int": func(columnType gorm.ColumnType) (dataType string) {
			if cn := columnType.Name(); cn == "deleted_at" || cn == "deletedAt" || cn == "DeletedAt" {
				return "soft_delete.DeletedAt"
			}
			if nullable, ok := columnType.Nullable(); nullable && ok {
				return "*int"
			}
			return "int"
		},

		"bigint": func(columnType gorm.ColumnType) (dataType string) {
			if cn := columnType.Name(); cn == "deleted_at" || cn == "deletedAt" || cn == "DeletedAt" {
				return "soft_delete.DeletedAt"
			}
			if nullable, ok := columnType.Nullable(); nullable && ok {
				return "*int64"
			}
			return "int64"
		},

		"varchar": func(columnType gorm.ColumnType) (dataType string) {
			if nullable, ok := columnType.Nullable(); nullable && ok {
				return "*string"
			}
			return "string"
		},

		"char": func(columnType gorm.ColumnType) (dataType string) {
			if nullable, ok := columnType.Nullable(); nullable && ok {
				return "*string"
			}
			return "string"
		},
		"json": func(columnType gorm.ColumnType) (dataType string) {
			if nullable, ok := columnType.Nullable(); nullable && ok {
				return "*string"
			}
			return "string"
		},
	}
}

type JsonTag struct {
	Replace string
	Append  []string
	Add     []map[string]string
}

func jsonTagName() map[string]JsonTag {
	return map[string]JsonTag{
		"created_at": {Append: []string{"omitzero"}, Add: []map[string]string{{"time_format": "sql_datetime"}}},
		"updated_at": {Append: []string{"omitzero"}, Add: []map[string]string{{"time_format": "sql_datetime"}}},
	}
}
