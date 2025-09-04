package cho

import (
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"net/http"
	"regexp"
	"strings"
)

var (
	validate     = validator.New()
	reMultiSlash = regexp.MustCompile(`/+`)
)

type ExchangeMaker[T any] func(http.ResponseWriter, *http.Request) *T

type Cho[T any] struct {
	raw         chi.Router
	excMaker    ExchangeMaker[T]
	prefixStack []string
}

func (cho *Cho[T]) Start(port int) error {
	return http.ListenAndServe(fmt.Sprintf(":%d", port), cho.raw)
}

func NewCho[T any](em ExchangeMaker[T]) *Cho[T] {
	router := chi.NewRouter()
	return &Cho[T]{
		raw:      router,
		excMaker: em,
	}
}

func DefaultCho() *Cho[Exchange] {
	return NewCho(func(w http.ResponseWriter, r *http.Request) *Exchange {
		return &Exchange{
			Response: w,
			Request:  r,
		}
	})
}

func (cho *Cho[T]) adapter(handle func(*T)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		handle(cho.excMaker(w, r))
	}
}

func (cho *Cho[T]) Get(path string, handler func(*T)) {
	cho.raw.Get(cho.fullPath(path), cho.adapter(handler))
}

func (cho *Cho[T]) Post(path string, handler func(*T)) {
	cho.raw.Post(cho.fullPath(path), cho.adapter(handler))
}

func (cho *Cho[T]) Use(middleware func(*T, http.Handler)) {
	wrap := func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			middleware(cho.excMaker(w, r), next)
		}
		return http.HandlerFunc(fn)
	}
	cho.raw.Use(wrap)
}

func (cho *Cho[T]) RawUse(mw ...func(next http.Handler) http.Handler) {
	cho.raw.Use(mw...)
}

func (cho *Cho[T]) fullPath(path string) string {
	return normalizePath(strings.Join(cho.prefixStack, "/") + path)
}

func (s *Cho[T]) Group(prefix string, fn func()) {
	s.prefixStack = append(s.prefixStack, normalizePath(prefix))
	fn()
	s.prefixStack = s.prefixStack[:len(s.prefixStack)-1]
}

func (s *Cho[T]) Static(path string, dir string) {
	s.raw.Mount(path, http.StripPrefix(path, http.FileServer(http.Dir(dir))))
}

func (s *Cho[T]) Mount(path string, h http.Handler) {
	s.raw.Mount(path, h)
}

func normalizePath(path string) string {
	path = "/" + strings.Trim(path, "/")
	return reMultiSlash.ReplaceAllString(path, "/")
}
