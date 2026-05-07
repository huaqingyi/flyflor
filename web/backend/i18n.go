package main

import (
	"fmt"
	"os"
	"strings"
)

// Language represents the supported languages
type Language string

const (
	LanguageEnglish Language = "en"
	LanguageChinese Language = "zh"
)

// current language (default: English)
var currentLang Language = LanguageEnglish

// TranslationKey represents a translation key used for i18n
type TranslationKey string

const (
	AppTooltip         TranslationKey = "AppTooltip"
	MenuOpen           TranslationKey = "MenuOpen"
	MenuOpenTooltip    TranslationKey = "MenuOpenTooltip"
	MenuAbout          TranslationKey = "MenuAbout"
	MenuAboutTooltip   TranslationKey = "MenuAboutTooltip"
	MenuVersion        TranslationKey = "MenuVersion"
	MenuVersionTooltip TranslationKey = "MenuVersionTooltip"
	MenuGitHub         TranslationKey = "MenuGitHub"
	MenuDocs           TranslationKey = "MenuDocs"
	MenuRestart        TranslationKey = "MenuRestart"
	MenuRestartTooltip TranslationKey = "MenuRestartTooltip"
	MenuQuit           TranslationKey = "MenuQuit"
	MenuQuitTooltip    TranslationKey = "MenuQuitTooltip"
	Exiting            TranslationKey = "Exiting"
	DocUrl             TranslationKey = "DocUrl"
)

// Translation tables
// Chinese translations intentionally contain Han script
//
//nolint:gosmopolitan
var translations = map[Language]map[TranslationKey]string{
	LanguageEnglish: {
		AppTooltip:         "%s - Web Console",
		MenuOpen:           "Open Console",
		MenuOpenTooltip:    "Open Flyflor console in browser",
		MenuAbout:          "About",
		MenuAboutTooltip:   "About Flyflor",
		MenuVersion:        "Version: %s",
		MenuVersionTooltip: "Current version number",
		MenuGitHub:         "Runtime Architecture",
		MenuDocs:           "Design Reference",
		MenuRestart:        "Restart Service",
		MenuRestartTooltip: "Restart Gateway service",
		MenuQuit:           "Quit",
		MenuQuitTooltip:    "Exit Flyflor",
		Exiting:            "Exiting Flyflor...",
		DocUrl:             "https://chatgpt.com/share/69fc3c71-be2c-83ea-ab29-e48fc0c6fe8d",
	},
	LanguageChinese: {
		AppTooltip:         "%s - Web Console",
		MenuOpen:           "打开控制台",
		MenuOpenTooltip:    "在浏览器中打开 Flyflor 控制台",
		MenuAbout:          "关于",
		MenuAboutTooltip:   "关于 Flyflor",
		MenuVersion:        "版本: %s",
		MenuVersionTooltip: "当前版本号",
		MenuGitHub:         "运行时架构",
		MenuDocs:           "设计参考",
		MenuRestart:        "重启服务",
		MenuRestartTooltip: "重启核心服务",
		MenuQuit:           "退出",
		MenuQuitTooltip:    "退出 Flyflor",
		Exiting:            "正在退出 Flyflor...",
		DocUrl:             "https://chatgpt.com/share/69fc3c71-be2c-83ea-ab29-e48fc0c6fe8d",
	},
}

// SetLanguage sets the current language
func SetLanguage(lang string) {
	lang = strings.ToLower(strings.TrimSpace(lang))

	// Extract language code before first underscore or dot
	// e.g., "en_US.UTF-8" -> "en", "zh_CN" -> "zh"
	if idx := strings.IndexAny(lang, "_."); idx > 0 {
		lang = lang[:idx]
	}

	if lang == "zh" || lang == "zh-cn" || lang == "chinese" {
		currentLang = LanguageChinese
	} else {
		currentLang = LanguageEnglish
	}
}

// GetLanguage returns the current language
func GetLanguage() Language {
	return currentLang
}

// T translates a key to the current language
func T(key TranslationKey, args ...any) string {
	if trans, ok := translations[currentLang][key]; ok {
		if len(args) > 0 {
			return fmt.Sprintf(trans, args...)
		}
		return trans
	}
	return string(key)
}

// Initialize i18n from environment variable
func init() {
	if lang := os.Getenv("LANG"); lang != "" {
		SetLanguage(lang)
	}
}
