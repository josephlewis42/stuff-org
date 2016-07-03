package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
)

const (
	kStaticResource = "/static/"
	kComponentImage = "/img/"
)

type ImageHandler struct {
	store      StuffStore
	template   *TemplateRenderer
	imgPath    string
	staticPath string
}

func AddImageHandler(store StuffStore, template *TemplateRenderer, imgPath string, staticPath string) {
	handler := &ImageHandler{
		store:      store,
		template:   template,
		imgPath:    imgPath,
		staticPath: staticPath,
	}
	http.Handle(kComponentImage, handler) // Serve an component image or fallback.
	http.Handle(kStaticResource, handler) // serve a static resource

	// With serving robots.txt, image-handler should probably be named
	// static handler.
	http.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		sendResource(staticPath+"/robots.txt", "", w)
	})
}

func (h *ImageHandler) ServeHTTP(out http.ResponseWriter, req *http.Request) {
	switch {
	case strings.HasPrefix(req.URL.Path, kComponentImage):
		prefix_len := len(kComponentImage)
		requested := req.URL.Path[prefix_len:]
		h.serveComponentImage(requested, out, req)
	default:
		h.serveStatic(out, req)
	}
}

// Create a synthetic representation of component from information given
// in the component.
func (h *ImageHandler) serveGeneratedComponentImage(component *Component, category string, value string,
	out http.ResponseWriter) bool {
	// If we got a category string, it takes precedence
	if len(category) == 0 && component != nil {
		category = component.Category
	}
	switch category {
	case "Resistor":
		return serveResistorImage(component, value, h.template, out)
	case "Diode (D)":
		return h.template.Render(out, "category-Diode.svg", component)
	case "LED":
		return h.template.Render(out, "category-LED.svg", component)
	case "Capacitor (C)":
		return h.template.Render(out, "category-Capacitor.svg", component)
	}
	return false
}

func (h *ImageHandler) servePackageImage(component *Component, out http.ResponseWriter) bool {
	if component == nil || component.Footprint == "" {
		return false
	}
	return h.template.Render(out,
		"package-"+component.Footprint+".svg", component)
}

func (h *ImageHandler) serveComponentImage(requested string, out http.ResponseWriter, r *http.Request) {
	path := h.imgPath + "/" + requested + ".jpg"
	if _, err := os.Stat(path); err == nil { // we have an image.
		sendResource(path, h.staticPath+"/fallback.png", out)
		return
	}
	// No image, but let's see if we can do something from the
	// component
	if comp_id, err := strconv.Atoi(requested); err == nil {
		component := h.store.FindById(comp_id)
		category := r.FormValue("c") // We also allow these if available
		value := r.FormValue("v")
		if (component != nil || len(category) > 0 || len(value) > 0) &&
			h.serveGeneratedComponentImage(component, category, value, out) {
			return
		}
		if h.servePackageImage(component, out) {
			return
		}
	}
	// Use fallback-resource straight away to get short cache times.
	sendResource("", h.staticPath+"/fallback.png", out)
}

func (h *ImageHandler) serveStatic(out http.ResponseWriter, r *http.Request) {
	prefix_len := len("/static/")
	resource := r.URL.Path[prefix_len:]
	sendResource(h.staticPath+"/"+resource, "", out)
}

func sendResource(local_path string, fallback_resource string, out http.ResponseWriter) {
	cache_time := 900
	header_addon := ""
	content, _ := ioutil.ReadFile(local_path)
	if content == nil && fallback_resource != "" {
		local_path = fallback_resource
		content, _ = ioutil.ReadFile(local_path)
		cache_time = 10 // fallbacks might change more often.
		out.WriteHeader(http.StatusNotFound)
		header_addon = ",must-revalidate"
	}
	out.Header().Set("Cache-Control", fmt.Sprintf("max-age=%d%s", cache_time, header_addon))
	switch {
	case strings.HasSuffix(local_path, ".png"):
		out.Header().Set("Content-Type", "image/png")
	case strings.HasSuffix(local_path, ".css"):
		out.Header().Set("Content-Type", "text/css")
	case strings.HasSuffix(local_path, ".svg"):
		out.Header().Set("Content-Type", "image/svg+xml")
	case strings.HasSuffix(local_path, ".txt"):
		out.Header().Set("Content-Type", "text/plain")
	default:
		out.Header().Set("Content-Type", "image/jpg")
	}

	out.Write(content)
}
