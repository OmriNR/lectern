package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"golang.org/x/term"
)

const site = "http://localhost:8080"
const service = "moodle_mobile_app"

func ws(fn, token string, params map[string]string) ([]byte, error) {
	q := url.Values{}
	q.Set("wsfunction", fn)
	q.Set("moodlewsrestformat", "json")

	if token != "" {
		q.Set("wstoken", token)
	}

	for k, v := range params {
		q.Set(k, v)
	}

	resp, err := http.PostForm(site+"/webservice/rest/server.php", q)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if len(body) > 0 && body[0] == '{' {
		var e struct {
			Exception string `json:"exception"`
			ErrorCode string `json:"errorcode"`
			Message   string `json:"message"`
		}

		if json.Unmarshal(body, &e) == nil && e.Exception != "" {
			return nil, fmt.Errorf("moodle: %s: %s", e.ErrorCode, e.Message)
		}
	}

	return body, nil
}

func login(username, password string) (string, error) {
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

func main() {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Username: ")
	username, _ := reader.ReadString('\n')
	username = strings.TrimSpace(username)

	fmt.Print("Password: ")
	passwordBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		fmt.Println("failed to read password:", err)
		os.Exit(1)
	}
	password := string(passwordBytes)

	token, err := login(username, password)
	if err != nil {
		fmt.Println("User does not exist or invalid credentials.")
		os.Exit(1)
	}

	raw, err := ws("core_user_get_users_by_field", token, map[string]string{
		"field":     "username",
		"values[0]": username,
	})
	if err != nil {
		fmt.Println("lookup failed:", err)
		os.Exit(1)
	}

	var users []struct {
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
	if err := json.Unmarshal(raw, &users); err != nil {
		fmt.Println("bad user data:", err)
		fmt.Println(string(raw))
		os.Exit(1)
	}

	if len(users) == 0 {
		fmt.Println("User does not exist.")
		os.Exit(1)
	}

	u := users[0]
	fmt.Println("\nUser found:")
	fmt.Printf("  id:         %d\n", u.ID)
	fmt.Printf("  username:   %s\n", u.Username)
	fmt.Printf("  full name:  %s\n", u.FullName)
	fmt.Printf("  email:      %s\n", u.Email)
	fmt.Printf("  department: %s\n", u.Department)
	fmt.Printf("  suspended:  %t\n", u.Suspended)
	fmt.Printf("  confirmed:  %t\n", u.Confirmed)

	raw, err = ws("core_enrol_get_users_courses", token, map[string]string{
		"userid": fmt.Sprint(u.ID),
	})
	if err != nil {
		fmt.Println("courses lookup failed:", err)
		os.Exit(1)
	}

	var courses []struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(raw, &courses); err != nil {
		fmt.Println("bad courses data:", err)
		fmt.Println(string(raw))
		os.Exit(1)
	}

	fmt.Printf("  courses:    %d\n", len(courses))
}
