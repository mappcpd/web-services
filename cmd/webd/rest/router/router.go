package router

import (
	"fmt"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/rs/cors"

	"github.com/mappcpd/web-services/cmd/webd/rest/router/handlers"
	"github.com/mappcpd/web-services/cmd/webd/rest/router/routes"
	"strings"
)

const (
	v1AuthBase    = "/v1/auth"
	v1MemberBase  = "/v1/m"
	v1AdminBase   = "/v1/a"
	v1GeneralBase = "/v1/g"
	v1ReportBase  = "/v1/r"
)

// Start fires up the router that handles requests to REST api endpoints
func Start(port string) {

	// Router
	r := mux.NewRouter()

	// Ping and preflight, no middleware required
	r.Methods("GET").Path("/").HandlerFunc(handlers.Index)
	r.Methods("OPTIONS").HandlerFunc(handlers.Preflight)
	//r.Methods("OPTIONS").Path("/").HandlerFunc(handlers.Preflight)

	// Auth sub-router, no middleware required
	rAuth := routes.AuthSubRouter(v1AuthBase)
	r.PathPrefix(v1AuthBase).Handler(rAuth)

	// Admin sub-router and middleware
	rAdmin := routes.AdminSubRouter(v1AdminBase)        // add router...
	rAdminMiddleware := routes.AdminMiddleware(rAdmin)  // ...plus middleware...
	r.PathPrefix(v1AdminBase).Handler(rAdminMiddleware) // ...and add to main router

	// Reports sub-router, todo: add middleware to reports router
	rReports := routes.ReportSubRouter(v1ReportBase)
	r.PathPrefix(v1ReportBase).Handler(rReports)

	// Member sub-router
	rMember := routes.MemberSubRouter(v1MemberBase)
	rMemberMiddleware := routes.MemberMiddleware(rMember)
	r.PathPrefix(v1MemberBase).Handler(rMemberMiddleware)

	// General sub-router
	rGeneral := routes.GeneralSubRouter(v1GeneralBase)
	rGeneralMiddleware := routes.GeneralMiddleware(rGeneral)
	r.PathPrefix(v1GeneralBase).Handler(rGeneralMiddleware)

	// CORS handler - needed to add OptionsPassThrough for preflight requests which use OPTIONS http method
	//handler := cors.Default().Handler(r)
	// Todo... tighten this up - not sure if needed  with preflightHandler??
	// todo: seem to have sorted this in the graphql handler so can possible remove the Preflight handler
	// in favour of the same set tup in graphql
	handler := cors.New(cors.Options{
		AllowedOrigins:     []string{"*"},
		AllowedMethods:     []string{"*"},
		AllowedHeaders:     []string{"*"},
		OptionsPassthrough: true,
	}).Handler(r)

	// strip port number if included in the env var, so we can add it again ;)
	host := strings.Join(strings.Split(os.Getenv("MAPPCPD_API_URL"), ":")[:2], "")
	fmt.Println("REST server listening at", host+":"+port)
	http.ListenAndServe(":"+port, handler)
}
