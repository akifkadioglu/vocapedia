package i18n

import (
	"log"
	"net/http"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

var bundle *i18n.Bundle

const (
	CONTEXT = "localizer"
)

func InitI18n() {
	bundle = i18n.NewBundle(language.Turkish)

	bundle.LoadMessageFile("pkg/i18n/de.json")
	bundle.LoadMessageFile("pkg/i18n/en.json")
	bundle.LoadMessageFile("pkg/i18n/es.json")
	bundle.LoadMessageFile("pkg/i18n/fr.json")
	bundle.LoadMessageFile("pkg/i18n/tr.json")
	bundle.LoadMessageFile("pkg/i18n/zh.json")
	log.Println("i18n is ready")
}

func Bundle() *i18n.Bundle {
	return bundle
}

func Localizer(r *http.Request, key string) string {
	localizer := r.Context().Value(CONTEXT).(*i18n.Localizer)
	val, err := localizer.Localize(&i18n.LocalizeConfig{MessageID: key})
	if err != nil {
		return key
	} else {
		return val
	}
}
