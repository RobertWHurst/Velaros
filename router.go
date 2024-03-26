package scramjet

import (
	"net/http"

	"github.com/RobertWHurst/navaros"
)

type Router struct{}

func NewRouter() *Router {
	return &Router{}
}

func (r *Router) Handle(ctx *navaros.Context) {

}

func (r *Router) ServeHTTP(res http.ResponseWriter, req *http.Request) {

}
