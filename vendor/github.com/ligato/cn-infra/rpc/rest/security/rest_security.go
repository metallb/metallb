// Copyright (c) 2018 Cisco and/or its affiliates.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:generate protoc --proto_path=model/access-security --gogo_out=model/access-security model/access-security/accesssecurity.proto

package security

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/gorilla/mux"
	"github.com/ligato/cn-infra/logging"
	access "github.com/ligato/cn-infra/rpc/rest/security/model/access-security"
	"github.com/pkg/errors"
	"github.com/unrolled/render"
	"golang.org/x/crypto/bcrypt"
)

const (
	// AuthHeaderKey helps to obtain authorization header matching the field in a request
	AuthHeaderKey = "authorization"
	// Admin constant, used to define admin security group and user
	admin = "admin"
)

const (
	// Returns login page where credentials may be put. Redirects to authenticate, and if successful, moves to index.
	login = "/login"
	// URL key for logout, invalidates current token.
	logout = "/logout"
	// Authentication page, validates credentials and if successful, returns a token or writes a cookie to a browser
	authenticate = "/authenticate"
	// Cookie name identifier
	cookieName = "ligato-rest"
)

// Default value to sign the token, if not provided from config file
var signature = "secret"

// Default expiration time for token/cookie
var defaultExpTime = time.Hour

// AuthenticatorAPI provides methods for handling permissions
type AuthenticatorAPI interface {
	// AddPermissionGroup adds new permission group. PG is defined by name and a set of URL keys. User with
	// permission group enabled has access to that set of keys. PGs with duplicated names are skipped.
	AddPermissionGroup(group ...*access.PermissionGroup)

	// Validate serves as middleware used while registering new HTTP handler. For every request, token
	// and permission group is validated.
	Validate(provider http.HandlerFunc) http.HandlerFunc
}

// Settings defines fields required to instantiate authenticator
type Settings struct {
	// Authentication database, default implementation is used if not set
	AuthStore AuthenticationDB
	// List of registered users
	Users []access.User
	// Expiration time (token claim). If not set, default value of 1 hour will be used.
	ExpTime time.Duration
	// Cost value used to hash user passwords
	Cost int
	// Custom token signature. If not set, default value will be used.
	Signature string
}

// Credentials struct represents simple user login input
type credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Authenticator keeps information about users, permission groups and tokens and processes it
type authenticator struct {
	log logging.Logger

	// Router instance automatically registers login/logout REST API handlers if authentication is enabled
	router    *mux.Router
	formatter *render.Render

	// User database keeps all known users with permissions and hashed password. Users are loaded from
	// HTTP config file
	// TODO add option to register users
	userDb AuthenticationDB
	// Permission database is a map of name/permissions and bound URLs
	groupDb map[string][]*access.PermissionGroup_Permissions

	// Token claims
	expTime time.Duration
}

// NewAuthenticator prepares new instance of authenticator.
func NewAuthenticator(router *mux.Router, ctx *Settings, log logging.Logger) AuthenticatorAPI {
	a := &authenticator{
		router: router,
		log:    log,
		formatter: render.New(render.Options{
			IndentJSON: true,
		}),
		groupDb: make(map[string][]*access.PermissionGroup_Permissions),
		expTime: ctx.ExpTime,
	}

	// Authentication store
	if ctx.AuthStore != nil {
		a.userDb = ctx.AuthStore
	} else {
		a.userDb = CreateDefaultAuthDB()
	}

	// Set token signature
	signature = ctx.Signature
	if a.expTime == 0 {
		a.expTime = defaultExpTime
		a.log.Debugf("Token expiration time claim not set, defaulting to 1 hour")
	}

	// Hash of default admin password, hashed with cost 10
	hash := "$2a$10$q5s1LP7xbCJWJlLet1g/h.rGrsHtciILps90bNRdJ.6DRekw9b.zK"
	if err := a.userDb.AddUser(admin, hash, []string{admin}); err != nil {
		a.log.Errorf("failed to add admin user: %v", err)
	}

	for _, user := range ctx.Users {
		if user.Name == admin {
			a.log.Errorf("rejected to create user-defined account named 'admin'")
			continue
		}
		if err := a.userDb.AddUser(user.Name, user.PasswordHash, user.Permissions); err != nil {
			a.log.Errorf("failed to add user %s: %v", user.Name, err)
			continue
		}
		a.log.Debug("Registered user %s, permissions: %v", user.Name, user.Permissions)
	}

	// Admin-group, available by default and always enabled for all URLs
	a.groupDb[admin] = []*access.PermissionGroup_Permissions{}

	a.registerSecurityHandlers()

	return a
}

// AddPermissionGroup adds new permission group.
func (a *authenticator) AddPermissionGroup(group ...*access.PermissionGroup) {
	for _, newPermissionGroup := range group {
		if _, ok := a.groupDb[newPermissionGroup.Name]; ok {
			a.log.Warnf("permission group %s already exists, skipped")
			continue
		}
		a.log.Debugf("added HTTP permission group %s", newPermissionGroup.Name)
		a.groupDb[newPermissionGroup.Name] = newPermissionGroup.Permissions
	}
}

// Validate the request
func (a *authenticator) Validate(provider http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		// Token may be accessed via cookie, or from authentication header
		tokenString, errCode, err := a.getTokenStringFromRequest(req)
		if err != nil {
			a.formatter.Text(w, errCode, err.Error())
			return
		}
		// Retrieve token object from raw string
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := jwt.GetSigningMethod(token.Header["alg"].(string)).(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("error parsing token")
			}
			return []byte(signature), nil
		})
		if err != nil {
			errStr := fmt.Sprintf("500 internal server error: %s", err)
			a.formatter.Text(w, http.StatusInternalServerError, errStr)
			return
		}
		// Validate token claims
		if token.Claims != nil {
			if err := token.Claims.Valid(); err != nil {
				errStr := fmt.Sprintf("401 Unauthorized: %v", err)
				a.formatter.Text(w, http.StatusUnauthorized, errStr)
				return
			}
		}
		// Validate token itself
		if err := a.validateToken(token, req.URL.Path, req.Method); err != nil {
			errStr := fmt.Sprintf("401 Unauthorized: %v", err)
			a.formatter.Text(w, http.StatusUnauthorized, errStr)
			return
		}

		provider.ServeHTTP(w, req)
	})
}

// Register authenticator-wide security handlers
func (a *authenticator) registerSecurityHandlers() {
	a.router.HandleFunc(login, a.loginHandler).Methods(http.MethodGet, http.MethodPost)
	a.router.HandleFunc(authenticate, a.authenticationHandler).Methods(http.MethodPost)
	a.router.HandleFunc(logout, a.logoutHandler).Methods(http.MethodPost)
}

// Login handler shows simple page to log in
func (a *authenticator) loginHandler(w http.ResponseWriter, req *http.Request) {
	// GET returns login page. Submit redirects to authenticate.
	if req.Method == http.MethodGet {
		r := render.New(render.Options{
			Directory:  "templates",
			Asset:      Asset,
			AssetNames: AssetNames,
		})
		r.HTML(w, http.StatusOK, "login", nil)
	} else {
		// POST decodes provided credentials
		credentials := &credentials{}
		decoder := json.NewDecoder(req.Body)
		err := decoder.Decode(&credentials)
		if err != nil {
			errStr := fmt.Sprintf("500 internal server error: failed to decode json: %v", err)
			a.formatter.Text(w, http.StatusInternalServerError, errStr)
			return
		}
		token, errCode, err := a.getTokenFor(credentials)
		if err != nil {
			a.formatter.Text(w, errCode, err.Error())
			return
		}

		// Returns token string.
		a.formatter.Text(w, http.StatusOK, token)
	}
}

// Authentication handler verifies credentials from login page (GET) and writes cookie with token
func (a *authenticator) authenticationHandler(w http.ResponseWriter, req *http.Request) {
	// Read name and password from the form (if accessed from browser)
	credentials := &credentials{
		Username: req.FormValue("name"),
		Password: req.FormValue("password"),
	}
	token, errCode, err := a.getTokenFor(credentials)
	if err != nil {
		a.formatter.Text(w, errCode, err.Error())
		return
	}
	// Writes cookie with token.
	http.SetCookie(w, &http.Cookie{
		Name:   cookieName,
		Path:   "/",
		MaxAge: int(a.expTime.Seconds()),
		Value:  token,
		Secure: false,
	})
	// Automatically move to index page.
	target := "/"
	http.Redirect(w, req, target, http.StatusMovedPermanently)
}

// Removes token endpoint from the DB. During processing, token will not be found and will be considered as invalid.
func (a *authenticator) logoutHandler(w http.ResponseWriter, req *http.Request) {
	decoder := json.NewDecoder(req.Body)
	var credentials credentials
	err := decoder.Decode(&credentials)
	if err != nil {
		errStr := fmt.Sprintf("500 internal server error: failed to decode json: %v", err)
		a.formatter.Text(w, http.StatusInternalServerError, errStr)
		return
	}

	a.userDb.SetLogoutTime(credentials.Username)
	a.log.Debugf("user %s was logged out", credentials.Username)
}

// Read raw token from request.
func (a *authenticator) getTokenStringFromRequest(req *http.Request) (result string, errCode int, err error) {
	// Try to read header, validate it if exists.
	authHeader := req.Header.Get(AuthHeaderKey)
	if authHeader != "" {
		bearerToken := strings.Split(authHeader, " ")
		if len(bearerToken) != 2 {
			return "", http.StatusUnauthorized, fmt.Errorf("401 Unauthorized: invalid authorization token")
		}
		// Parse token header constant
		if bearerToken[0] != "Bearer" {
			return "", http.StatusUnauthorized, fmt.Errorf("401 Unauthorized: invalid authorization header")
		}
		return bearerToken[1], 0, nil
	}
	a.log.Debugf("Authentication header not found (err: %v)", err)

	// Otherwise read cookie
	cookie, err := req.Cookie(cookieName)
	if err == nil && cookie != nil {
		return cookie.Value, 0, nil
	}
	a.log.Debugf("Authentication cookie not found (err: %v)", err)

	return "", http.StatusUnauthorized, fmt.Errorf("401 Unauthorized: authorization required")
}

// Get token for credentials
func (a *authenticator) getTokenFor(credentials *credentials) (string, int, error) {
	name, errCode, err := a.validateCredentials(credentials)
	if err != nil {
		return "", errCode, err
	}
	claims := jwt.StandardClaims{
		Audience:  name,
		ExpiresAt: a.expTime.Nanoseconds(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(signature))
	if err != nil {
		return "", http.StatusInternalServerError, fmt.Errorf("500 internal server error: failed to sign token: %v", err)
	}
	a.userDb.SetLoginTime(name)
	a.log.Debugf("user %s was logged in", name)

	return tokenString, 0, nil
}

// Validates credentials, returns name and error code/message if invalid
func (a *authenticator) validateCredentials(credentials *credentials) (string, int, error) {
	user, err := a.userDb.GetUser(credentials.Username)
	if err != nil {
		return "", http.StatusUnauthorized, errors.Errorf("401 unauthorized: user name or password is incorrect")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(credentials.Password)); err != nil {
		return credentials.Username, http.StatusUnauthorized, fmt.Errorf("401 unauthorized: user name or password is incorrect")
	}
	return credentials.Username, 0, nil
}

// Validates token itself and permissions
func (a *authenticator) validateToken(token *jwt.Token, url, method string) error {
	var userName string
	// Read audience from the token
	switch v := token.Claims.(type) {
	case jwt.MapClaims:
		var ok bool
		if userName, ok = v["aud"].(string); !ok {
			return fmt.Errorf("failed to validate token claims audience")
		}
	case jwt.StandardClaims:
		userName = v.Audience
	default:
		return fmt.Errorf("failed to validate token claims")
	}
	loggedOut, err := a.userDb.IsLoggedOut(userName)
	if err != nil {
		return fmt.Errorf("failed to validate token: %v", err)
	}
	if loggedOut {
		// User logged out
		token.Valid = false
		return fmt.Errorf("invalid token")
	}
	user, err := a.userDb.GetUser(userName)
	if err != nil {
		return fmt.Errorf("failed to validate token: %v", err)
	}
	// Do not check for permissions if user is admin
	if userIsAdmin(user) {
		return nil
	}
	perms := a.getPermissionsForURL(url, method)
	for _, userPerm := range user.Permissions {
		for _, perm := range perms {
			if userPerm == perm {
				return nil
			}
		}
	}

	return fmt.Errorf("not permitted")
}

// Returns all permission groups provided URL/Method is allowed for
func (a *authenticator) getPermissionsForURL(url, method string) []string {
	var groups []string
	for groupName, permissions := range a.groupDb {
		for _, permissions := range permissions {
			// Check URL
			if permissions.Url == url {
				// Check allowed methods
				for _, allowed := range permissions.AllowedMethods {
					if allowed == method {
						groups = append(groups, groupName)
					}
				}
			}
		}
	}
	return groups
}

// Checks user admin permission
func userIsAdmin(user *User) bool {
	for _, permission := range user.Permissions {
		if permission == admin {
			return true
		}
	}
	return false
}
