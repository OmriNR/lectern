package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const service = "moodle_mobile_app"

type MoodleClient struct {
	host  string
	token string
	http  *http.Client
	user  *User
}

type moodleError struct {
	Exception string `json:"exception"`
	ErrorCode string `json:"errorcode"`
	Message   string `json:"message"`
}

func (e *moodleError) Error() string {
	return fmt.Sprintf("moodle: %s: %s", e.ErrorCode, e.Message)
}

// New constructs a Client for the given Moodle base URL. Call Login to
// authenticate before making any other requests.
func New(host string) *MoodleClient {
	return &MoodleClient{
		host: host,
		http: &http.Client{},
	}
}

// Login authenticates with the given credentials and stores the resulting
// token and user on the client, returning an error if authentication fails.
func (c *MoodleClient) Login(username, password string) error {
	token, err := c.requestToken(username, password)
	if err != nil {
		return err
	}
	c.token = token

	user, err := c.GetUserByUsername(username)
	if err != nil {
		return err
	}
	if user == nil {
		return fmt.Errorf("moodle: user %q not found", username)
	}
	c.user = user

	return nil
}

// User returns the currently connected user, or nil if Login has not
// been called successfully yet.
func (c *MoodleClient) User() *User {
	return c.user
}

// Host returns the Moodle base URL this client talks to.
func (c *MoodleClient) Host() string {
	return c.host
}

// RestoreSession sets the token and user on the client without going
// through Login, e.g. when restoring a session saved in the local config.
func (c *MoodleClient) RestoreSession(token string, user *User) {
	c.token = token
	c.user = user
}

func (c *MoodleClient) requestToken(username, password string) (string, error) {
	resp, err := c.http.PostForm(c.host+"/login/token.php", url.Values{
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

func (c *MoodleClient) call(fn string, params url.Values) ([]byte, error) {
	if params == nil {
		params = url.Values{}
	}

	params.Set("wstoken", c.token)
	params.Set("wsfunction", fn)
	params.Set("moodlewsrestformat", "json")

	resp, err := c.http.PostForm(c.host+"/webservice/rest/server.php", params)
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

func (c *MoodleClient) GetUserByUsername(username string) (*User, error) {
	params := url.Values{}
	params.Set("field", "username")
	// Moodle usernames are canonically lowercase: login/token.php accepts
	// any case, but core_user_get_users_by_field's parameter validation
	// rejects a non-lowercase value outright.
	params.Set("values[0]", strings.ToLower(username))

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
	ID                       int       `json:"id"`
	ShortName                string    `json:"shortname"`
	FullName                 string    `json:"fullname"`
	DisplayName              string    `json:"displayname"`
	IDNumber                 string    `json:"idnumber"`
	Visible                  int       `json:"visible"`
	Summary                  string    `json:"summary"`
	SummaryFormat            int       `json:"summaryformat"`
	Format                   string    `json:"format"`
	ShowGrades               bool      `json:"showgrades"`
	Lang                     string    `json:"lang"`
	EnableCompletion         bool      `json:"enablecompletion"`
	Category                 int       `json:"category"`
	Progress                 *float64  `json:"progress"`
	StartDate                int64     `json:"startdate"`
	EndDate                  int64     `json:"enddate"`
	Marker                   int       `json:"marker"`
	LastAccess               int64     `json:"lastaccess"`
	IsFavourite              bool      `json:"isfavourite"`
	Hidden                   bool      `json:"hidden"`
	ShowActivityDates        bool      `json:"showactivitydates"`
	ShowCompletionConditions bool      `json:"showcompletionconditions"`
	TimeModified             int64     `json:"timemodified"`
	Sections                 []Section `json:"sections"`
}

type ModuleContent struct {
	Type         string `json:"type"`
	FileName     string `json:"filename"`
	FilePath     string `json:"filepath"`
	FileURL      string `json:"fileurl"`
	FileSize     int64  `json:"filesize"`
	TimeModified int64  `json:"timemodified"`
}

type Module struct {
	ID       int             `json:"id"`
	Name     string          `json:"name"`
	ModName  string          `json:"modname"`
	Contents []ModuleContent `json:"contents,omitempty"`
}

type Section struct {
	ID      int      `json:"id"`
	Name    string   `json:"name"`
	Summary string   `json:"summary"`
	Section int      `json:"section"`
	Visible bool     `json:"uservisible"`
	Modules []Module `json:"modules"`
}

func (c *MoodleClient) GetUserCourses() ([]Course, error) {
	if c.user == nil {
		return nil, fmt.Errorf("moodle: not connected")
	}

	params := url.Values{}
	params.Set("userid", strconv.Itoa(c.user.ID))

	body, err := c.call("core_enrol_get_users_courses", params)
	if err != nil {
		return nil, err
	}

	var courses []Course
	if err := json.Unmarshal(body, &courses); err != nil {
		return nil, fmt.Errorf("courses: bad json: %w\n%s", err, string(body))
	}

	for i := range courses {
		sections, err := c.GetCourseSections(courses[i].ID)
		if err != nil {
			return nil, fmt.Errorf("course %d: %w", courses[i].ID, err)
		}
		courses[i].Sections = sections
	}

	return courses, nil
}

func (c *MoodleClient) GetCourseSections(courseID int) ([]Section, error) {
	params := url.Values{}
	params.Set("courseid", strconv.Itoa(courseID))

	body, err := c.call("core_course_get_contents", params)
	if err != nil {
		return nil, err
	}

	var sections []Section
	if err := json.Unmarshal(body, &sections); err != nil {
		return nil, fmt.Errorf("course sections: bad json: %w\n%s", err, string(body))
	}

	// core_course_get_contents doesn't expose mod_assign intro attachments
	// (unlike page/resource/folder, assign has no "contents" of its own) -
	// fetch them separately and splice them into the matching module.
	attachmentsByModuleID, err := c.getAssignmentIntroAttachments(courseID)
	if err != nil {
		return nil, err
	}

	for i := range sections {
		for j := range sections[i].Modules {
			mod := &sections[i].Modules[j]
			if mod.ModName != "assign" {
				continue
			}
			if attachments, ok := attachmentsByModuleID[mod.ID]; ok {
				mod.Contents = attachments
			}
		}
	}

	return sections, nil
}

// getAssignmentIntroAttachments returns, per course-module ID, the files
// attached to an assignment's description (mod_assign_get_assignments is
// the only web service that exposes these).
func (c *MoodleClient) getAssignmentIntroAttachments(courseID int) (map[int][]ModuleContent, error) {
	params := url.Values{}
	params.Set("courseids[0]", strconv.Itoa(courseID))

	body, err := c.call("mod_assign_get_assignments", params)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Courses []struct {
			Assignments []struct {
				CMID             int             `json:"cmid"`
				IntroAttachments []ModuleContent `json:"introattachments"`
			} `json:"assignments"`
		} `json:"courses"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("assignments: bad json: %w\n%s", err, string(body))
	}

	byModuleID := make(map[int][]ModuleContent)
	for _, course := range resp.Courses {
		for _, a := range course.Assignments {
			byModuleID[a.CMID] = a.IntroAttachments
		}
	}

	return byModuleID, nil
}

// DownloadFile fetches the raw content of a Moodle file URL (e.g. a
// ModuleContent.FileURL), authenticating with the client's token.
func (c *MoodleClient) DownloadFile(fileURL string) ([]byte, error) {
	u, err := url.Parse(fileURL)
	if err != nil {
		return nil, fmt.Errorf("download: bad file url %q: %w", fileURL, err)
	}

	q := u.Query()
	q.Set("token", c.token)
	u.RawQuery = q.Encode()

	resp, err := c.http.Get(u.String())
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", fileURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: unexpected status %s", fileURL, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("download %s: reading body: %w", fileURL, err)
	}

	return body, nil
}

// syntheticContentName is the filename Moodle's mod_page always uses for
// the auto-generated file that mirrors a page's HTML body (see
// page_export_contents() in mod/page/lib.php). It isn't a real uploaded
// file, so it's excluded - but real files attached within a page (or any
// other module) keep their own filenames and are still included.
const syntheticContentName = "index.html"

func (s Section) GetFiles() []ModuleContent {
	var files []ModuleContent

	for _, mod := range s.Modules {
		for _, content := range mod.Contents {
			if content.FileURL == "" || content.FileName == syntheticContentName {
				continue
			}
			files = append(files, content)
		}
	}
	return files
}
