// Package session issues and verifies this backend's own session token.
//
// The provider's identity token proves who somebody is exactly once, at sign-in
// (pkg/oidc). Everything after that is authenticated by a token this server
// signed with a secret only this server holds, which is what makes a session
// verifiable without a round trip to Apple or Google on every request.
//
// The token is symmetric (HS256): the only party that has to verify it is the
// party that issued it, so a key pair would add an operational moving part and
// buy nothing. Nothing here logs — a session token is a bearer credential
// (004-security.md).
package session

import (
	"errors"
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/google/uuid"
)

const (
	// MinSecretBytes is the shortest secret that may sign a session. HS256's
	// security is the secret's entropy, and anything a person can type is
	// below this line — which is the point: a short secret is a weak one
	// whether it came from an operator or from a habit.
	MinSecretBytes = 32

	// issuer and audience are this backend naming itself in its own tokens.
	// They are constants rather than configuration because both sides of the
	// check are this process: making them configurable would only create a way
	// for the two halves to disagree.
	issuer   = "binder-api"
	audience = "binder-app"

	// DefaultTTL is how long a session is accepted for.
	//
	// There is no revocation list in the MVP, so this duration *is* the window
	// in which a stolen token still works, and it is the only thing that closes
	// it. A day is the compromise: shorter means a user re-runs the provider's
	// sign-in sheet during ordinary use, because there is no refresh token yet.
	// Shrinking it is an environment variable, not a code change.
	DefaultTTL = 24 * time.Hour

	// clockSkew is how far this server's clock may lag behind the one that
	// issued the token. Both are this process, so the only real source of skew
	// is a rolling deploy across machines; go-jose's own default of a minute
	// would keep accepting an expired session for that minute.
	clockSkew = 30 * time.Second
)

var (
	// ErrSessionRejected means the token is not a session this server issued
	// and is still accepting: a bad signature, the wrong issuer or audience, an
	// expired session, or a subject that is not a user id. It is one error for
	// all of them — every one is a request with no usable session, and telling
	// a caller which check failed is an oracle.
	ErrSessionRejected = errors.New("session token rejected")

	// ErrSecretMissing means no signing secret was configured. It is a startup
	// failure on purpose: there is deliberately no fallback secret, not even
	// for development, because a fallback is how a known key reaches
	// production (004-security.md).
	ErrSecretMissing = errors.New("session signing secret is not set")

	// ErrSecretTooShort means the configured secret is below MinSecretBytes.
	ErrSecretTooShort = errors.New("session signing secret is too short")
)

// Config is how a session signer is configured. LoadConfig reads it from the
// environment; a test builds it directly.
type Config struct {
	// Secret signs every session token. It is read from the environment and
	// validated at startup: env's `required` refuses a variable that is not
	// set, and New refuses one that is set to something too weak to sign with.
	Secret string `env:"SESSION_JWT_SECRET,required"`
	// TTL is how long an issued session is accepted for.
	TTL time.Duration `env:"SESSION_TTL" envDefault:"24h"`
}

// LoadConfig reads the session configuration from the environment.
//
// It is the exit-if-missing check the bootstrap makes: main returns this error
// and exits non-zero, so a process with no signing secret never reaches the
// point of serving a request. It does not log the secret, or its length, or
// anything else about it.
func LoadConfig() (Config, error) {
	cfg, err := env.ParseAs[Config]()
	if err != nil {
		return Config{}, fmt.Errorf("reading the session configuration: %w", err)
	}
	return cfg, nil
}

// Tokens issues and verifies session tokens. One instance serves the whole
// process; it holds no per-request state and is safe for concurrent use.
type Tokens struct {
	secret []byte
	ttl    time.Duration
	now    func() time.Time
}

// New returns a signer over cfg, refusing a secret that cannot safely sign.
//
// now is the clock, so a spec can watch a session expire without waiting for
// it; nil means time.Now.
func New(cfg Config, now func() time.Time) (*Tokens, error) {
	if cfg.Secret == "" {
		return nil, ErrSecretMissing
	}
	if len(cfg.Secret) < MinSecretBytes {
		return nil, fmt.Errorf("%w: it must be at least %d bytes", ErrSecretTooShort, MinSecretBytes)
	}

	ttl := cfg.TTL
	if ttl == 0 {
		ttl = DefaultTTL
	}
	if now == nil {
		now = time.Now
	}

	return &Tokens{secret: []byte(cfg.Secret), ttl: ttl, now: now}, nil
}

// Token is an issued session: the value the app sends back, and when it stops
// being accepted.
type Token struct {
	Value     string
	ExpiresAt time.Time
}

// Issue mints a session for userID.
func (t *Tokens) Issue(userID uuid.UUID) (Token, error) {
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.HS256, Key: t.secret}, nil)
	if err != nil {
		return Token{}, fmt.Errorf("building the session signer: %w", err)
	}

	issuedAt := t.now()
	expiresAt := issuedAt.Add(t.ttl)
	claims := jwt.Claims{
		Issuer:    issuer,
		Subject:   userID.String(),
		Audience:  jwt.Audience{audience},
		Expiry:    jwt.NewNumericDate(expiresAt),
		NotBefore: jwt.NewNumericDate(issuedAt),
		IssuedAt:  jwt.NewNumericDate(issuedAt),
		ID:        "",
	}

	value, err := jwt.Signed(signer).Claims(claims).Serialize()
	if err != nil {
		return Token{}, fmt.Errorf("signing the session: %w", err)
	}

	return Token{Value: value, ExpiresAt: expiresAt}, nil
}

// Verify returns the user a session token was issued to, or ErrSessionRejected.
func (t *Tokens) Verify(rawToken string) (uuid.UUID, error) {
	// HS256 and nothing else. Accepting a second algorithm here would accept a
	// token this server did not sign.
	parsed, err := jwt.ParseSigned(rawToken, []jose.SignatureAlgorithm{jose.HS256})
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: not a signed JWT: %w", ErrSessionRejected, err)
	}

	var claims jwt.Claims
	if err := parsed.Claims(t.secret, &claims); err != nil {
		return uuid.Nil, fmt.Errorf("%w: the signature does not verify: %w", ErrSessionRejected, err)
	}

	// This server always sets an expiry, so an absent one means the token was
	// not built here however well it verifies.
	if claims.Expiry == nil {
		return uuid.Nil, fmt.Errorf("%w: no expiry claim", ErrSessionRejected)
	}

	expected := jwt.Expected{
		Issuer:      issuer,
		Subject:     "",
		AnyAudience: jwt.Audience{audience},
		ID:          "",
		Time:        t.now(),
	}
	if err := claims.ValidateWithLeeway(expected, clockSkew); err != nil {
		return uuid.Nil, fmt.Errorf("%w: %w", ErrSessionRejected, err)
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: the subject is not a user id: %w", ErrSessionRejected, err)
	}

	return userID, nil
}
