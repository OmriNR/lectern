package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

const service = "moodle_mobile_app"

type Client struct {
	site  string
	token string
	http  *http.Client
}

type moodleError struct {
	Exception string `json:"exception"`
	ErrorCode string `json:"errorcode"`
	Message   string `json:"message"`
}

func (e *moodleError) Error() string {
	return fmt.Sprintf("moodle: %s: %s", e.ErrorCode, e.Message)
}

func Login(site, username, password string) (string, error) {
	resp, err := http.PostForm(site+"/login/token.php", url.Values{
		"username": {username},
		"password": {password},
		"service":  {service},
	})
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var result struct {
		Token     string `json:"token"`
		Error     string `json:"error"`
		ErrorCode string `json:"errorcode"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("bad login response: %w\n%s", err, string(body))
	}

	if result.Token == "" {
		return "", fmt.Errorf("%s", result.Error)
	}

	return result.Token, nil
}

func New(site, token string) *Client {
	return &Client{
		site:  site,
		token: token,
		http:  &http.Client{},
	}
}

func (c *Client) call(fn string, params url.Values) ([]byte, error) {
	if params == nil {
		params = url.Values{}
	}

	params.Set("wstoken", c.token)
	params.Set("wsfunction", fn)
	params.Set("moodlewsrestformat", "json")

	resp, err := c.http.PostForm(c.site+"/webservice/rest/server.php", params)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", fn, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%s: reading body: %w", fn, err)
	}

	if len(body) > 0 && body[0] == '{' {
		var me moodleError

		if json.Unmarshal(body, &me) == nil && me.Exception != "" {
			return nil, &me
		}
	}

	return body, nil
}

type User struct {
	ID         int    `json:"id"`
	Username   string `json:"username"`
	FullName   string `json:"fullname"`
	Email      string `json:"email"`
	Department string `json:"department"`
	FirstName  string `json:"firstname"`
	LastName   string `json:"lastname"`
	Suspended  bool   `json:"suspended"`
	Confirmed  bool   `json:"confirmed"`
}

func (c *Client) GetUserByUsername(username string) (*User, error) {
	params := url.Values{}
	params.Set("field", "username")
	params.Set("values[0]", username)

	body, err := c.call("core_user_get_users_by_field", params)
	if err != nil {
		return nil, err
	}

	var users []User
	if err := json.Unmarshal(body, &users); err != nil {
		return nil, fmt.Errorf("users_by_field: bad json: %w\n%s", err, string(body))
	}

	if len(users) == 0 {
		return nil, nil
	}

	return &users[0], nil
}

type Course struct {
	ID int `json:"id"`
}

func (c *Client) GetUserCourses(userID int) ([]Course, error) {
	params := url.Values{}
	params.Set("userid", strconv.Itoa(userID))

	body, err := c.call("core_enrol_get_users_courses", params)
	if err != nil {
		return nil, err
	}

	var courses []Course
	if err := json.Unmarshal(body, &courses); err != nil {
		return nil, fmt.Errorf("courses: bad json: %w\n%s", err, string(body))
	}

	return courses, nil
}
