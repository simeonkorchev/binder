package main_test

import (
	"context"
	"encoding/json"
	"net/http"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/simeonkorchev/binder/internal/user/session"
)

// pathParameter matches the {binderId}-style placeholders in an OpenAPI path,
// so a spec can turn every documented route into one it can actually call.
var pathParameter = regexp.MustCompile(`\{[^}]+\}`)

// openAPIPaths is the subset of the document these specs read: the routes, and
// which methods each one answers.
type openAPIPaths struct {
	Paths map[string]map[string]struct{} `json:"paths"`
}

var _ = Describe("binderd", func() {
	// The specs below share one running server: starting a process, opening a
	// connection pool and binding a socket per spec would pay that cost a dozen
	// times to observe the same one server. Ordered is what makes a BeforeAll
	// legal, and nothing here mutates state another spec reads.
	Describe("serving", Ordered, func() {
		var (
			server *serverProcess
			// actorID is a user seeded into the database, so the authenticated
			// specs exercise a real row rather than only the 404 path.
			actorID = uuid.New()
			token   string
		)

		BeforeAll(func() {
			seedUser(actorID)

			tokens, err := session.New(session.Config{Secret: sessionSecret, TTL: time.Hour}, nil)
			Expect(err).NotTo(HaveOccurred())
			issued, err := tokens.Issue(actorID)
			Expect(err).NotTo(HaveOccurred())
			token = issued.Value

			server = start(defaultEnv())
		})

		AfterAll(func() { server.stop() })

		When("the liveness probe is called", func() {
			It("answers over the socket", func() {
				resp := server.do(http.MethodGet, "/health", "", "")

				Expect(resp.StatusCode).To(Equal(http.StatusOK))
				Expect(readBody(resp)).To(ContainSubstring(`"status":"ok"`))
			})
		})

		When("every route the server documents is called", func() {
			It("answers each one itself, without falling through to the router's 404", func() {
				for path, methods := range documentedRoutes() {
					for method := range methods {
						target := pathParameter.ReplaceAllString(path, uuid.NewString())
						resp := server.do(method, target, token, bodyFor(method))
						body := readBody(resp)

						// A registered route is one the server answers on its
						// own terms: any status its handlers or Huma's
						// validation chose. The two failures this is here to
						// catch are the route not being mounted at all — the
						// mux's plain-text "404 page not found" — and a handler
						// blowing up behind it.
						Expect(resp.StatusCode).To(BeNumerically("<", http.StatusInternalServerError),
							"%s %s answered %d: %s", method, target, resp.StatusCode, body)
						Expect(body).NotTo(ContainSubstring("404 page not found"),
							"%s %s is documented but not mounted", method, target)
					}
				}
			})
		})

		When("an endpoint that needs a session is called without one", func() {
			It("answers 401 rather than acting for nobody", func() {
				resp := server.do(http.MethodGet, "/me", "", "")

				Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})

		When("an endpoint that needs a session is called with one", func() {
			It("acts as the user the token names, all the way to the database", func() {
				resp := server.do(http.MethodGet, "/me", token, "")
				body := readBody(resp)

				Expect(resp.StatusCode).To(Equal(http.StatusOK), body)
				Expect(body).To(ContainSubstring(actorID.String()))
			})
		})

		When("a public endpoint is called without a session", func() {
			It("answers it, because browsing the marketplace needs no account", func() {
				resp := server.do(http.MethodGet, "/listings", "", "")

				Expect(resp.StatusCode).To(Equal(http.StatusOK), readBody(resp))
			})
		})

		When("a request writes and another reads it back", func() {
			It("carries one session through both, over the socket", func() {
				created := server.do(http.MethodPost, "/binders", token, `{"name":"Smoke"}`)
				Expect(created.StatusCode).To(Equal(http.StatusCreated), readBody(created))

				listed := server.do(http.MethodGet, "/binders", token, "")
				Expect(listed.StatusCode).To(Equal(http.StatusOK))
				Expect(readBody(listed)).To(ContainSubstring("Smoke"))
			})
		})
	})

	When("a required configuration value is missing", func() {
		It("exits non-zero without serving", func() {
			env := defaultEnv()
			delete(env, "SESSION_JWT_SECRET")

			server := launch(env)
			Expect(server.cmd.Wait()).NotTo(Succeed())
			Expect(server.logs.String()).NotTo(ContainSubstring("binderd is listening"))
			// The reason is reported, and the secret itself never is — there is
			// nothing to leak here, but the message must stay that way.
			Expect(server.logs.String()).To(ContainSubstring("SESSION_JWT_SECRET"))
		})
	})

	When("the OpenAPI document is asked for", func() {
		It("is produced with no configuration and no database at all", func() {
			// No environment whatsoever: `make gen-spec` runs on a machine that
			// has neither a database nor a signing secret, and a document that
			// needed one would make the contract ungenerable in CI.
			generate := exec.CommandContext(context.Background(), binaryPath, "-openapi")
			generate.Env = []string{}

			document, err := generate.Output()
			Expect(err).NotTo(HaveOccurred())

			var paths openAPIPaths
			Expect(json.Unmarshal(document, &paths)).To(Succeed())
			Expect(paths.Paths).To(HaveKey("/health"))
			Expect(paths.Paths).To(HaveKey("/binders"))
			Expect(paths.Paths).To(HaveKey("/cards"))
			Expect(paths.Paths).To(HaveKey("/listings"))
			Expect(paths.Paths).To(HaveKey("/me"))
		})
	})
})

// documentedRoutes reads the routes out of the binary's own OpenAPI document, so
// the "every route answers" spec cannot fall behind a domain that registers a
// new endpoint.
func documentedRoutes() map[string]map[string]struct{} {
	GinkgoHelper()

	generate := exec.CommandContext(context.Background(), binaryPath, "-openapi")
	generate.Env = []string{}

	document, err := generate.Output()
	Expect(err).NotTo(HaveOccurred())

	var paths openAPIPaths
	Expect(json.Unmarshal(document, &paths)).To(Succeed())
	Expect(paths.Paths).NotTo(BeEmpty())

	routes := make(map[string]map[string]struct{}, len(paths.Paths))
	for path, methods := range paths.Paths {
		routes[path] = make(map[string]struct{}, len(methods))
		for method := range methods {
			routes[path][strings.ToUpper(method)] = struct{}{}
		}
	}
	return routes
}

// bodyFor is the request body a smoke call sends. An empty object is enough: the
// point is that the route is mounted and answers for itself, and an endpoint
// that wants more says so with a 422 of its own.
func bodyFor(method string) string {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch:
		return "{}"
	default:
		return ""
	}
}

// seedUser inserts the account the authenticated specs act as. It writes through
// the same database the server was pointed at, because what is under test is the
// server's own read of it.
func seedUser(id uuid.UUID) {
	GinkgoHelper()

	db, err := sqlx.Connect("pgx", databaseURL)
	Expect(err).NotTo(HaveOccurred())
	defer func() { Expect(db.Close()).To(Succeed()) }()

	_, err = db.ExecContext(context.Background(),
		`INSERT INTO users (id, auth_provider, auth_subject) VALUES ($1, 'google', $2)`,
		id, "smoke-"+id.String())
	Expect(err).NotTo(HaveOccurred())
}
