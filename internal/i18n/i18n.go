package i18n

import (
	"strings"
	"sync"
)

type Lang string

const (
	ZhCN Lang = "zh-CN"
	En   Lang = "en"
)

var (
	mu      sync.RWMutex
	current = En
)

var en = map[string]string{
	"tray.tooltip":           "pdf-watcher: watch folder (top-level only) → Zotero",
	"tray.addWatch":          "Add watch folder",
	"tray.watched":           "Watched folders",
	"tray.pauseDir":          "Pause",
	"tray.resumeDir":         "Resume",
	"tray.removeDir":         "Remove",
	"tray.pauseAll":          "Pause watching",
	"tray.resumeAll":         "Resume watching",
	"tray.openConfig":        "Open config",
	"tray.openLogs":          "Open log folder",
	"tray.help":              "Help",
	"tray.openGuideZh":       "User guide (中文)",
	"tray.openGuideEn":       "User guide (English)",
	"tray.exportCollections": "Export Zotero collections",
	"tray.autoStart":         "Start with Windows",
	"tray.language":          "Language",
	"tray.lang.zh":           "中文",
	"tray.lang.en":           "English",
	"tray.quit":              "Quit",
	"tray.emptyWatch":        "(none)",
	"msg.alwaysAllow":        "Please choose Always Allow in the Zotero dialog",
	"msg.denied":             "Zotero write authorization denied",
	"msg.zoteroDown":         "Zotero local API unreachable; queuing",
}

var zh = map[string]string{
	"tray.tooltip":           "pdf-watcher：监听文件夹（仅本层）→ Zotero",
	"tray.addWatch":          "添加监控文件夹",
	"tray.watched":           "已监控目录",
	"tray.pauseDir":          "暂停",
	"tray.resumeDir":         "启用",
	"tray.removeDir":         "删除",
	"tray.pauseAll":          "暂停监听",
	"tray.resumeAll":         "继续监听",
	"tray.openConfig":        "打开配置文件",
	"tray.openLogs":          "打开日志文件夹",
	"tray.help":              "帮助",
	"tray.openGuideZh":       "使用指南（中文）",
	"tray.openGuideEn":       "使用指南（English）",
	"tray.exportCollections": "导出 Zotero 文集",
	"tray.autoStart":         "开机自启",
	"tray.language":          "语言",
	"tray.lang.zh":           "中文",
	"tray.lang.en":           "English",
	"tray.quit":              "退出",
	"tray.emptyWatch":        "（无）",
	"msg.alwaysAllow":        "请在 Zotero 弹窗中选择 Always Allow",
	"msg.denied":             "Zotero 写入授权被拒绝",
	"msg.zoteroDown":         "Zotero 本地 API 不可用，任务已排队",
}

func Set(lang string) {
	mu.Lock()
	defer mu.Unlock()
	current = Resolve(lang)
}

func Get() Lang {
	mu.RLock()
	defer mu.RUnlock()
	return current
}

func Resolve(lang string) Lang {
	l := strings.TrimSpace(lang)
	switch {
	case l == "" || strings.EqualFold(l, "auto"):
		return detectSystem()
	case strings.HasPrefix(strings.ToLower(l), "zh"):
		return ZhCN
	default:
		return En
	}
}

func detectSystem() Lang {
	// Overridden on Windows via SetFromWindowsUI; default en.
	return En
}

func SetSystemLang(lang Lang) {
	mu.Lock()
	defer mu.Unlock()
	if current == En || current == ZhCN {
		// only used at startup before config language applied
	}
	_ = lang
}

func ApplyConfigOrSystem(cfgLang string, system Lang) {
	mu.Lock()
	defer mu.Unlock()
	if strings.TrimSpace(cfgLang) == "" {
		current = system
		return
	}
	current = Resolve(cfgLang)
}

func T(key string) string {
	mu.RLock()
	lang := current
	mu.RUnlock()
	if lang == ZhCN {
		if v, ok := zh[key]; ok {
			return v
		}
	}
	if v, ok := en[key]; ok {
		return v
	}
	return key
}
