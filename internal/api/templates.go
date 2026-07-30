package api

import (
	"html/template"
	"io"
	"log"
	"os"
	"path/filepath"
	"reflect"
)

// PageData 页面渲染上下文
type PageData struct {
	Title     string
	PageID    string
	Instances interface{}
	Rooms     interface{}
	Mods      interface{}
	FlashMsg  string
}

func countRunning(instances interface{}) int {
	rv := reflect.ValueOf(instances)
	if rv.Kind() != reflect.Slice { return 0 }
	count := 0
	for i := 0; i < rv.Len(); i++ {
		elem := rv.Index(i)
		if elem.Kind() == reflect.Ptr { elem = elem.Elem() }
		if field, ok := elem.Type().FieldByName("Status"); ok {
			if elem.FieldByIndex(field.Index).String() == "running" {
				count++
			}
		}
	}
	return count
}

func (s *Server) renderHTML(w io.Writer, tmplName string, data *PageData) {
	tmpl := template.New(tmplName + ".html")
	tmpl.Funcs(template.FuncMap{
		"statusClass": func(status string) string {
			switch status {
			case "running":  return "badge-running"
			case "stopped":  return "badge-stopped"
			case "starting": return "badge-starting"
			default:         return "badge-error"
			}
		},
		"pageActive": func(current, target string) string { if current == target { return "active" } else { return "" } },
		"lenVal": func(v interface{}) int { if v == nil { return 0 } else { return reflect.ValueOf(v).Len() } },
		"runningCount": func(i interface{}) int { return countRunning(i) },
		"stoppedCount": func(i interface{}) int { rv := reflect.ValueOf(i); if rv.Kind() != reflect.Slice { return 0 } else { return rv.Len() - countRunning(i) } },
	})

	candidates := []string{
		"templates/" + tmplName + ".html",
		filepath.Join(".", "templates", tmplName+".html"),
		filepath.Join(filepath.Dir(os.Args[0]), "templates", tmplName+".html"),
	}

	for _, p := range candidates {
		b, err := os.ReadFile(p)
		if err != nil { continue }
		if _, err := tmpl.Parse(string(b)); err == nil { break }
	}

	if data.FlashMsg != "" && tmplName == "dashboard" {
		// dashboard 需要显示 msg
	}

	data.PageID = tmplName
	if err := tmpl.Execute(w, data); err != nil {
		log.Printf("[template] 执行模板失败 %s: %v", tmplName, err)
		w.Write([]byte("<h1>模板执行失败</h1><pre>" + err.Error() + "</pre>"))
	}
}
