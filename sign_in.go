package tweet_words

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/garyburd/go-oauth/oauth"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"sync"
	"text/template"
	"time"
)

var HttpAddr = flag.String("addr", "0.0.0.0:8080", "HTTP server address")

var OauthClient = oauth.Client{
	TemporaryCredentialRequestURI: "https://api.twitter.com/oauth/request_token",
	ResourceOwnerAuthorizationURI: "https://api.twitter.com/oauth/authorize",
	TokenRequestURI:               "https://api.twitter.com/oauth/access_token",
}

var (
	// secrets maps credential tokens to credential secrets. A real application will use a database to store credentials.
	secretsMutex sync.Mutex
	Secrets      = map[string]string{}
)

var SigninOAuthClient oauth.Client

type templateData struct {
	KeywordsSearched []string
	DataStored       []TweetStore
}

type SSE struct {
	tweetInfoChan chan string
}

// authHandler reads the auth cookie and invokes a handler with the result.
type AuthHandler struct {
	handler  func(w http.ResponseWriter, r *http.Request, c *oauth.Credentials)
	optional bool
}

func Home() {
	fmt.Println("Home")
	http.Handle("/", &AuthHandler{handler: ServeHome, optional: true})
}

func ReadCredentials() {
	OauthClient.Credentials = oauth.Credentials{Conf["CONSUMER_KEY"], Conf["CONSUMER_SECRET"]}
}

func PutCredentials(cred *oauth.Credentials) {
	fmt.Println("PutCredentials")
	secretsMutex.Lock()
	defer secretsMutex.Unlock()
	Secrets[cred.Token] = cred.Secret
}

func GetCredentials(token string) *oauth.Credentials {
	fmt.Println("GetCredentials")
	secretsMutex.Lock()
	defer secretsMutex.Unlock()
	if secret, ok := Secrets[token]; ok {
		return &oauth.Credentials{Token: token, Secret: secret}
	}
	return nil
}

func DeleteCredentials(token string) {
	fmt.Println("DeleteCredentials")
	secretsMutex.Lock()
	defer secretsMutex.Unlock()
	delete(Secrets, token)
}

// serveSignin gets the OAuth temp credentials and redirects the user to the
// Twitter's authentication page.
func ServeSignin(w http.ResponseWriter, r *http.Request) {
	fmt.Println("ServeSignin")
	callback := "http://" + r.Host + "/callback"
	tempCred, err := SigninOAuthClient.RequestTemporaryCredentials(nil, callback, nil)
	if err != nil {
		http.Error(w, "Error getting temp cred, "+err.Error(), 500)
		return
	}
	PutCredentials(tempCred)
	http.Redirect(w, r, SigninOAuthClient.AuthorizationURL(tempCred, nil), 302)
}

// serveOAuthCallback handles callbacks from the OAuth server.
func ServeOAuthCallback(w http.ResponseWriter, r *http.Request) {
	fmt.Println("ServeOAuthCallback")
	tempCred := GetCredentials(r.FormValue("oauth_token"))
	if tempCred == nil {
		http.Error(w, "Unknown oauth_token.", 500)
		return
	}
	DeleteCredentials(tempCred.Token)
	tokenCred, _, err := OauthClient.RequestToken(nil, tempCred, r.FormValue("oauth_verifier"))
	if err != nil {
		http.Error(w, "Error getting request token, "+err.Error(), 500)
		return
	}
	PutCredentials(tokenCred)
	http.SetCookie(w, &http.Cookie{
		Name:     "auth",
		Path:     "/",
		HttpOnly: true,
		Value:    tokenCred.Token,
	})
	http.Redirect(w, r, "/", 302)
}

// serveLogout clears the authentication cookie.
func ServeLogout(w http.ResponseWriter, r *http.Request) {
	fmt.Println("ServeLogout")
	http.SetCookie(w, &http.Cookie{
		Name:     "auth",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
		Expires:  time.Now().Add(-1 * time.Hour),
	})
	http.Redirect(w, r, "/", 302)
}

func (h *AuthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Println("ServeHTTP")
	var cred *oauth.Credentials
	if c, _ := r.Cookie("auth"); c != nil {
		cred = GetCredentials(c.Value)
	}
	if cred == nil && !h.optional {
		http.Error(w, "Not logged in.", 403)
		return
	}
	h.handler(w, r, cred)
}

// apiGet issues a GET request to the Twitter API and decodes the response JSON to data.
func ApiGet(cred *oauth.Credentials, urlStr string, form url.Values, data interface{}) error {
	fmt.Println("ApiGet")
	resp, err := OauthClient.Get(nil, cred, urlStr, form)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return DecodeResponse(resp, data)
}

// apiPost issues a POST request to the Twitter API and decodes the response JSON to data.
func ApiPost(cred *oauth.Credentials, urlStr string, form url.Values, data interface{}) error {
	fmt.Println("ApiPost")
	resp, err := OauthClient.Post(nil, cred, urlStr, form)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return DecodeResponse(resp, data)
}

// decodeResponse decodes the JSON response from the Twitter API.
func DecodeResponse(resp *http.Response, data interface{}) error {
	fmt.Println("DecodeResponse")
	if resp.StatusCode != 200 {
		p, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("get %s returned status %d, %s", resp.Request.URL, resp.StatusCode, p)
	}
	return json.NewDecoder(resp.Body).Decode(data)
}

// respond responds to a request by executing the html template t with data.
func Respond(w http.ResponseWriter, t *template.Template, data interface{}) {
	fmt.Println("Respond")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.Execute(w, data); err != nil {
		log.Print(err)
	}
}

func ServeHome(w http.ResponseWriter, r *http.Request, cred *oauth.Credentials) {
	fmt.Println("ServeHome")
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if cred == nil {
		Respond(w, HomeLoggedOutTmpl, "loggedout")
	} else {
		user := User{cred.Token, cred.Secret, nil}
		StoreUser(user)
		templData := GetKeywordsList(user)
		Respond(w, HomeTmpl, templData)
	}
}

var (
	HomeLoggedOutTmpl, _ = template.ParseFiles("../index.html")
	HomeTmpl, _          = template.ParseFiles("../mainPage.html")
)

func StoreKeywordServ(w http.ResponseWriter, r *http.Request) {

	keyValue := r.URL.Query()

	c, _ := r.Cookie("auth")
	cred := GetCredentials(c.Value)
	user := User{cred.Token, cred.Secret, nil}

	StoreKeywords(keyValue["keyword"][0], user)

	// TODO (vin18) rename
	x := 10 * time.Minute
	xyz := url.Values{}
	xyz.Set("track", keyValue["keyword"][0])
	xyz.Add("language", "en")
	tweetChan := StoreTweets(xyz, x, keyValue["keyword"][0])

	// TODO (vin18) Currently, if same keyword is searched again
	// a panic occurs as same handle cannot be registerd twice

	// TODO (vin18) Add a mechanism to detect if any user is accepting events

	templData := GetKeywordsList(user)
	http.Handle("/"+url.QueryEscape(keyValue["keyword"][0]), &SSE{tweetInfoChan: tweetChan})
	Respond(w, HomeTmpl, templData)
}

func (b *SSE) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	// Set the headers related to event streaming.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	for {

		// Read from our messageChan.
		msg := <-b.tweetInfoChan

		// Write to the ResponseWriter, `w`.
		fmt.Fprintf(w, "data:%s\n\n", msg)
		f.Flush()
	}
}

func GetKeywordsList(user User) templateData {
	return templateData{KeywordsSearched: GetKeywords(user)}
}

func GetKeywordsServ(w http.ResponseWriter, r *http.Request) {

	c, _ := r.Cookie("auth")
	cred := GetCredentials(c.Value)
	user := User{cred.Token, cred.Secret, nil}

	keyValue := r.URL.Query()
	templData := GetKeywordsList(user)
	tweets := GetTweets(keyValue["keyword"][0])
	templData.DataStored = tweets
	Respond(w, HomeTmpl, templData)
}
