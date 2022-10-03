package server

import (
	"embed"
	"github.com/ShutovAndrey/weblocation/internal/services/logger"
	"github.com/ShutovAndrey/weblocation/internal/services/provider"
	"html/template"
	"net/http"
	"strings"
)

//go:embed templates
var index embed.FS

//go:embed static
var styles embed.FS

var tpl *template.Template

func indexHandler(w http.ResponseWriter, r *http.Request) {

	t, err := template.ParseFS(index, "templates/index.html")
	if err != nil {
		logger.Error(err)
	}

	ip := strings.Split(r.RemoteAddr, ":")[0]

	t.Execute(w, provider.GetDataByIP(ip))
}

func Create() error {

	var stylesFS = http.FS(styles)
	fs := http.FileServer(stylesFS)

	// Serve static files
	http.Handle("/static/", fs)

	http.HandleFunc("/", indexHandler)

	err := http.ListenAndServe("", nil)
	if err != nil {
		return err
	}
	return nil

}
