package spec

import (
	"miwu-go/internal/miot/model"
)

type LanguageCode string

type languageType int

type languageMap struct {
	Data map[LanguageCode]model.LanguageMap `json:"data"`
}

const (
	LanguageChinese LanguageCode = "zh_cn"
	LanguageEnglish LanguageCode = "en"
)

func initTranslations(attr *model.Specification) {
	for i := range attr.Services {
		svc := &attr.Services[i]
		svc.Translation = svc.Description

		initProperties(svc.Properties)
		initActions(svc.Actions)
	}
}

func initProperties(props *[]model.Property) {
	if props == nil {
		return
	}
	for i := range *props {
		prop := &(*props)[i]
		prop.Translation = prop.Description

		initValueList(prop.ValueList)
	}
}

func initValueList(values *[]model.Value) {
	if values == nil {
		return
	}
	for i := range *values {
		val := &(*values)[i]
		val.Translation = val.Description
	}
}

func initActions(actions *[]model.Action) {
	if actions == nil {
		return
	}
	for i := range *actions {
		act := &(*actions)[i]
		act.Translation = act.Description
	}
}
