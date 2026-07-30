package api

import (
	"html/template"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"sync"
	"time"
)

var (
	globalTmpl *template.Template
	tmplLock   sync.RWMutex
)

func countRunning(instances interface{}) int {
	rv := reflect.ValueOf(instances)
	if rv.Kind() != reflect.Slice {
		return 0
	}
	count := 0
	for i := 0; i < rv.Len(); i++ {
		elem := rv.Index(i)
		if elem.Kind() == reflect.Ptr {
			elem = elem.Elem()
		}
		if f, ok := elem.Type().FieldByName("Status"); ok {
			if elem.FieldByIndex(f.Index).String() == "running" {
				count++
			}
		}
	}
	return count
}

func countStopped(instances interface{}) int {
	rv := reflect.ValueOf(instances)
	if rv.Kind() != reflect.Slice {
		return 0
	}
	return rv.Len() - countRunning(instances)
}

func lenVal(v interface{}) int {
	if v == nil {
		return 0
	}
	return reflect.ValueOf(v).Len()
}

func pageActive(current, target string) string {
	if current == target {
		return "active"
	}
	return ""
}

func statusClass(s string) string {
	switch s {
	case "running":
		return "badge-running"
	case "stopped":
		return "badge-stopped"
	case "starting":
		return "badge-starting"
	default:
		return "badge-error"
	}
}

func formatTime(t interface{}) string {
	if tm, ok := t.(time.Time); ok {
		if tm.IsZero() {
			return "—"
		}
		return tm.Format("2006-01-02 15:04")
	}
	return ""
}

func formatInt(v interface{}) string {
	if i, ok := v.(int); ok {
		return strconv.Itoa(i)
	}
	if i, ok := v.(int64); ok {
		return strconv.FormatInt(i, 10)
	}
	return ""
}

func initTemplates(templateDir string) error {
	funcMap := template.FuncMap{
		"pageActive":   pageActive,
		"statusClass":  statusClass,
		"lenVal":       lenVal,
		"runningCount": countRunning,
		"stoppedCount": countStopped,
		"formatTime":   formatTime,
		"formatInt":    formatInt,
	}

	pattern := filepath.Join(templateDir, "*.html")
	files, err := filepath.Glob(pattern)
	if err != nil || len(files) == 0 {
		return nil
	}

	var t *template.Template
	for _, f := range files {
		if t == nil {
			t = template.Must(template.New(filepath.Base(f)).Funcs(funcMap).ParseFiles(f))
		} else {
			t, _ = t.Funcs(funcMap).ParseFiles(f)
		}
	}

	tmplLock.Lock()
	globalTmpl = t
	tmplLock.Unlock()
	return nil
}

func InitTemplatesFromDir(templateDir string) error {
	return initTemplates(templateDir)
}

func (s *Server) loadTemplate(name string) (*template.Template, error) {
	tmplLock.RLock()
	defer tmplLock.RUnlock()
	if globalTmpl == nil {
		return nil, os.ErrNotExist
	}
	return globalTmpl, nil
}
