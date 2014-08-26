package main

import (
	"code.google.com/p/gcfg"
	"fmt"
	"net/http"
	"net/url"
)

var paywallLogins = map[string]func(*http.Client) error{
	//	"telegraph.co.uk":      loginTelegraph,
	"thesun.co.uk":         loginSun,
	"thetimes.co.uk":       loginTimes,
	"thesundaytimes.co.uk": loginSundayTimes,
	//"ft.com":               loginFT,
}

func loginTelegraph(c *http.Client) error {
	conf := struct {
		Telegraph struct {
			Email    string
			Password string
		}
	}{}
	err := gcfg.ReadFileInto(&conf, "paywalls/telegraph.gcfg")
	if err != nil {
		return err
	}

	details := &conf.Telegraph

	loginURL := "https://auth.telegraph.co.uk/sam-ui/login.htm"
	postData := url.Values{}
	postData.Set("email", details.Email)
	postData.Set("password", details.Password)
	//postData.Set("remember", "true")
	resp, err := c.PostForm(loginURL, postData)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// returns 200 on failure, showing the login page
	// or
	// 301 upon success, redirecting to account page:
	// "https://auth.telegraph.co.uk/customer-portal/myaccount/index.html"
	if resp.StatusCode != http.StatusMovedPermanently {
		return fmt.Errorf("wrong email/password?")
	}

	urlStr := resp.Header.Get("Location")
	if urlStr != "https://auth.telegraph.co.uk/customer-portal/myaccount/index.html" {
		return fmt.Errorf("didn't redirect to expected location")
	}

	return nil
}

func loginSun(c *http.Client) error {

	conf := struct {
		TheSun struct {
			Username string
			Password string
		}
	}{}
	err := gcfg.ReadFileInto(&conf, "paywalls/thesun.gcfg")
	if err != nil {
		return err
	}

	details := &conf.TheSun

	loginURL := "https://login.thesun.co.uk/"
	successHost := "www.thesun.co.uk"
	failureHost := "login.thesun.co.uk"
	return loginNI(c, loginURL, successHost, failureHost, details.Username, details.Password)
}

func loginTimes(c *http.Client) error {
	conf := struct {
		TheTimes struct {
			Username string
			Password string
		}
	}{}
	err := gcfg.ReadFileInto(&conf, "paywalls/thetimes.gcfg")
	if err != nil {
		return err
	}

	details := &conf.TheTimes

	loginURL := "https://login.thetimes.co.uk/"
	successHost := "www.thetimes.co.uk"
	failureHost := "login.thetimes.co.uk"
	return loginNI(c, loginURL, successHost, failureHost, details.Username, details.Password)
}

func loginSundayTimes(c *http.Client) error {
	conf := struct {
		TheSundayTimes struct {
			Username string
			Password string
		}
	}{}
	err := gcfg.ReadFileInto(&conf, "paywalls/thesundaytimes.gcfg")
	if err != nil {
		return err
	}

	details := &conf.TheSundayTimes

	loginURL := "https://login.thesundaytimes.co.uk/"
	successHost := "www.thesundaytimes.co.uk"
	failureHost := "login.thesundaytimes.co.uk"
	return loginNI(c, loginURL, successHost, failureHost, details.Username, details.Password)
}

// common login for sun, times and sunday times
func loginNI(c *http.Client, loginURL, successHost, failureHost, username, password string) error {

	postData := url.Values{}
	postData.Set("username", username)
	postData.Set("password", password)
	//postData.Set("rememberMe", "on")
	resp, err := c.PostForm(loginURL, postData)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	//fmt.Printf("Ended up at: %s %s %d\n", resp.Request.Method, resp.Request.URL, resp.StatusCode)

	// on failure, just returns 200 and shows the login page again
	// on success, it redirects us through a whole _heap_ of other login pages
	// (presumably to collect cookies for thesun.ie, page3.com
	// scottishsun.com etc), then finally
	// leaves us with a successful 200 GET at the front page (eg "http://www.thesun.co.uk/sol/homepage/") (or possibly a 301  in the case of the sunday times)
	if resp.StatusCode != 200 && resp.StatusCode != 301 {
		return fmt.Errorf("unexpected http code (%d)", resp.StatusCode)
	}

	host := resp.Request.URL.Host
	switch host {
	case successHost: // eg "www.thetimes.co.uk":
		return nil // success!
	case failureHost: //"login.thetimes.co.uk":
		// could also check for "bad email/password" message on form
		return fmt.Errorf("bad username/password?")
	default:
		return fmt.Errorf("ended up at unexpected url (%s)", resp.Request.URL)
	}
}

func loginFT(c *http.Client) error {
	conf := struct {
		FT struct {
			Username string
			Password string
		}
	}{}
	err := gcfg.ReadFileInto(&conf, "paywalls/ft.gcfg")
	if err != nil {
		return err
	}

	details := &conf.FT

	loginURL := "https://registration.ft.com/registration/barrier/login"

	postData := url.Values{}
	postData.Set("username", details.Username)
	postData.Set("password", details.Password)
	//	postData.Set("rememberme", "on")
	resp, err := c.PostForm(loginURL, postData)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	//fmt.Printf("Ended up at: %s %s %d\n", resp.Request.Method, resp.Request.URL, resp.StatusCode)

	// upon success, redirects us on to "http://www.ft.com/home/uk"
	// upon failure, returns a 200, but leaves us on registration.ft.com

	switch resp.Request.URL.Host {
	case "www.ft.com":
		return nil
	case "registration.ft.com":
		return fmt.Errorf("bad username/password?")
	default:
		return fmt.Errorf("ended up at unexpected url (%s)", resp.Request.URL)
	}

}
