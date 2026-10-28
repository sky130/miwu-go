package spec

import (
	"testing"

	"miwu-go/internal/miot/model"
)

func testSpecification() *model.Specification {
	values := []model.Value{{Value: 0, Description: "off", Translation: "off"}}
	props := []model.Property{{IID: 1, Description: "power", ValueList: &values}}
	return &model.Specification{Services: []model.Service{{IID: 2, Properties: &props}}}
}

func TestInvalidTranslationIndicesKeepOriginalDescription(t *testing.T) {
	for _, key := range []string{"service:2:property:1:valuelist:-1", "service:2:property:1:valuelist:99", "service:2:garbage:1:valuelist:0", "service:2:property:1:valuelist:0:extra"} {
		attr := testSpecification()
		applyTranslations(attr, model.LanguageMap{key: "bad"})
		if got := (*(*attr.Services[0].Properties)[0].ValueList)[0].Translation; got != "off" {
			t.Fatalf("invalid translation changed value: %s", key)
		}
	}
}

func FuzzTranslationKeys(f *testing.F) {
	f.Add("service:2:property:1:valuelist:0")
	f.Add("service:2:property:1:valuelist:-1")
	f.Fuzz(func(t *testing.T, key string) {
		applyTranslations(testSpecification(), model.LanguageMap{key: "translation"})
	})
}
