// Seeds the local Moodle instance (see ../docker-compose.yml) with majors,
// categories, courses, lecture pages, and assignments with attached files.
//
// All the actual data lives here, in Go. runner.php is fixed, generic
// plumbing - it just reads whatever JSON this program produces and creates
// it inside Moodle via internal generator APIs (Moodle's web service layer
// has no supported way to create fully-configured activities, so this is
// the only reliable path).
//
// Usage: go run ./local-moodle/seed
package main

import (
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

//go:embed runner.php
var runnerPHP string

const containerName = "local-moodle_moodle_1"

type lecture struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

type assignment struct {
	Name                     string `json:"name"`
	Intro                    string `json:"intro"`
	DueDate                  int64  `json:"duedate"`
	AllowSubmissionsFromDate int64  `json:"allowsubmissionsfromdate"`
	FileName                 string `json:"filename"`
	FileContentBase64        string `json:"filecontentbase64"`
}

// resource is a mod_resource ("File") activity: a single downloadable file,
// used to seed the non-page, non-assignment file types (pdf, slides, code,
// video) that core_course_get_contents exposes directly.
type resource struct {
	Name              string `json:"name"`
	FileName          string `json:"filename"`
	FileContentBase64 string `json:"filecontentbase64"`
}

type course struct {
	ShortName   string       `json:"shortname"`
	FullName    string       `json:"fullname"`
	Summary     string       `json:"summary"`
	StartDate   int64        `json:"startdate"`
	Lectures    []lecture    `json:"lectures"`
	Assignments []assignment `json:"assignments"`
	Resources   []resource   `json:"resources"`
}

type category struct {
	Name    string   `json:"name"`
	Courses []course `json:"courses"`
}

type major struct {
	Name       string     `json:"name"`
	Categories []category `json:"categories"`
}

type seedUser struct {
	Username  string   `json:"username"`
	Password  string   `json:"password"`
	FirstName string   `json:"firstname"`
	LastName  string   `json:"lastname"`
	Email     string   `json:"email"`
	Courses   []string `json:"courses"`
}

type spec struct {
	Majors []major    `json:"majors"`
	Users  []seedUser `json:"users"`
}

func days(n int) int64 {
	return time.Now().Add(time.Duration(n) * 24 * time.Hour).Unix()
}

// bigDeadline is intentionally far out so seeded assignments never go stale
// while poking at the API.
const bigDeadlineDays = 365

func makeLecture(name, body string) lecture {
	return lecture{
		Name:    name,
		Content: fmt.Sprintf("<p>Lecture notes for <strong>%s</strong>.</p><p>%s</p>", name, body),
	}
}

func makeAssignment(courseShortName, name, instructions string) assignment {
	filename := strings.ReplaceAll(fmt.Sprintf("%s_%s.txt", courseShortName, name), " ", "_")
	filename = strings.ReplaceAll(filename, ":", "")
	content := fmt.Sprintf(
		"%s\nCourse: %s\n\nInstructions:\n%s\n\nSubmit your work through Moodle before the due date.\n",
		name, courseShortName, instructions,
	)
	return assignment{
		Name:                     name,
		Intro:                    fmt.Sprintf("<p>%s</p><p>See the attached file for full instructions.</p>", instructions),
		DueDate:                  days(bigDeadlineDays),
		AllowSubmissionsFromDate: days(-1),
		FileName:                 filename,
		FileContentBase64:        base64.StdEncoding.EncodeToString([]byte(content)),
	}
}

// minimalPDF returns a bare-bones but valid single-page PDF containing the
// given title text.
func minimalPDF(title string) []byte {
	return []byte(fmt.Sprintf(`%%PDF-1.4
1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj
2 0 obj<</Type/Pages/Kids[3 0 R]/Count 1>>endobj
3 0 obj<</Type/Page/Parent 2 0 R/MediaBox[0 0 300 150]/Contents 4 0 R/Resources<</Font<</F1 5 0 R>>>>>>endobj
4 0 obj<</Length 58>>
stream
BT /F1 14 Tf 20 100 Td (%s) Tj ET
endstream
endobj
5 0 obj<</Type/Font/Subtype/Type1/BaseFont/Helvetica>>endobj
trailer<</Size 6/Root 1 0 R>>
%%%%EOF
`, title))
}

// mockPresentation returns placeholder slide-deck content. It isn't a real
// OOXML .pptx (not worth the complexity for mock data) - it just exercises
// the download path for that file type.
func mockPresentation(title string) []byte {
	return []byte(fmt.Sprintf("Mock presentation: %s\n\n(placeholder content for testing file downloads)\n", title))
}

// mockPythonScript returns a small, genuinely valid Python script.
func mockPythonScript(title string) []byte {
	return []byte(fmt.Sprintf(`"""%s - mock script for lectern installation testing."""


def greet(name: str) -> str:
    return f"Hello, {name}!"


if __name__ == "__main__":
    print(greet("world"))
`, title))
}

// mockMP4 returns non-text binary placeholder content, useful for
// exercising the download path's handling of binary (not just text) files.
func mockMP4() []byte {
	data := make([]byte, 2048)
	for i := range data {
		data[i] = byte(i % 256)
	}
	return data
}

func makeResourceFile(courseShortName, name, filename string, content []byte) resource {
	fname := strings.ReplaceAll(fmt.Sprintf("%s_%s", courseShortName, filename), " ", "_")
	return resource{
		Name:              name,
		FileName:          fname,
		FileContentBase64: base64.StdEncoding.EncodeToString(content),
	}
}

// makeCourseResources returns one mock file per type we want lectern's
// download path exercised against: a pdf, a presentation, a Python source
// file, and a video.
func makeCourseResources(courseShortName string) []resource {
	return []resource{
		makeResourceFile(courseShortName, "Course Slides", "slides.pptx", mockPresentation(courseShortName+" - Course Slides")),
		makeResourceFile(courseShortName, "Reading Material", "reading.pdf", minimalPDF(courseShortName+" Reading Material")),
		makeResourceFile(courseShortName, "Example Script", "example.py", mockPythonScript(courseShortName+" Example Script")),
		makeResourceFile(courseShortName, "Lecture Recording", "recording.mp4", mockMP4()),
	}
}

func buildSpec() spec {
	return spec{
		Majors: []major{
			{
				Name: "Computer Science",
				Categories: []category{
					{
						Name: "Year 1",
						Courses: []course{
							{
								ShortName: "CS101",
								FullName:  "Introduction to Programming",
								Summary:   "Fundamentals of programming using Python: variables, control flow, functions, and basic data structures.",
								StartDate: days(-30),
								Lectures: []lecture{
									makeLecture("Lecture 1: What is Programming?", "An overview of what programs are and how computers run them."),
									makeLecture("Lecture 2: Variables and Control Flow", "Storing data and controlling execution with conditionals and loops."),
									makeLecture("Lecture 3: Functions and Scope", "Organizing code into reusable functions."),
								},
								Assignments: []assignment{
									makeAssignment("CS101", "Homework 1: Basic Syntax", "Write short programs demonstrating variables, conditionals, and loops."),
									makeAssignment("CS101", "Homework 2: Functions", "Implement a set of functions solving the given problems."),
								},
								Resources: makeCourseResources("CS101"),
							},
						},
					},
					{
						Name: "Year 2",
						Courses: []course{
							{
								ShortName: "CS201",
								FullName:  "Data Structures and Algorithms",
								Summary:   "Arrays, linked lists, trees, hash tables, sorting and searching algorithms, and complexity analysis.",
								StartDate: days(-30),
								Lectures: []lecture{
									makeLecture("Lecture 1: Arrays and Linked Lists", "Comparing contiguous and linked storage."),
									makeLecture("Lecture 2: Trees and Hash Tables", "Hierarchical and hashed data structures."),
								},
								Assignments: []assignment{
									makeAssignment("CS201", "Assignment 1: Implement a Linked List", "Implement a singly linked list with insert/delete/search."),
									makeAssignment("CS201", "Assignment 2: Sorting Algorithms", "Implement and compare quicksort and mergesort."),
								},
								Resources: makeCourseResources("CS201"),
							},
						},
					},
				},
			},
			{
				Name: "Mathematics",
				Categories: []category{
					{
						Name: "Year 1",
						Courses: []course{
							{
								ShortName: "MATH101",
								FullName:  "Calculus I",
								Summary:   "Limits, derivatives, and integrals of single-variable functions.",
								StartDate: days(-30),
								Lectures: []lecture{
									makeLecture("Lecture 1: Limits and Continuity", "The formal definition of a limit."),
									makeLecture("Lecture 2: Derivatives", "Rates of change and differentiation rules."),
								},
								Assignments: []assignment{
									makeAssignment("MATH101", "Problem Set 1: Limits", "Evaluate the given limits, showing all steps."),
									makeAssignment("MATH101", "Problem Set 2: Derivatives", "Differentiate the given functions."),
								},
								Resources: makeCourseResources("MATH101"),
							},
						},
					},
					{
						Name: "Year 2",
						Courses: []course{
							{
								ShortName: "MATH201",
								FullName:  "Linear Algebra",
								Summary:   "Vectors, matrices, linear transformations, eigenvalues and eigenvectors.",
								StartDate: days(21),
								Lectures: []lecture{
									makeLecture("Lecture 1: Vectors and Matrices", "Basic operations on vectors and matrices."),
								},
								Assignments: []assignment{
									makeAssignment("MATH201", "Problem Set 1: Matrix Operations", "Perform the given matrix operations by hand."),
								},
								Resources: makeCourseResources("MATH201"),
							},
						},
					},
				},
			},
			{
				Name: "Physics",
				Categories: []category{
					{
						Name: "Year 1",
						Courses: []course{
							{
								ShortName: "PHYS101",
								FullName:  "Classical Mechanics",
								Summary:   "Kinematics, Newton's laws, energy and momentum, rotational motion.",
								StartDate: days(-30),
								Lectures: []lecture{
									makeLecture("Lecture 1: Kinematics", "Describing motion without regard to its causes."),
									makeLecture("Lecture 2: Newton's Laws of Motion", "Force, mass, and acceleration."),
								},
								Assignments: []assignment{
									makeAssignment("PHYS101", "Problem Set 1: Kinematics", "Solve the given kinematics problems."),
									makeAssignment("PHYS101", "Problem Set 2: Forces", "Apply Newton's laws to the given scenarios."),
								},
								Resources: makeCourseResources("PHYS101"),
							},
						},
					},
				},
			},
			{
				Name: "Psychology",
				Categories: []category{
					{
						Name: "Year 1",
						Courses: []course{
							{
								ShortName: "PSYCH101",
								FullName:  "Introduction to Psychology",
								Summary:   "Foundational theories and methods in psychology: cognition, behavior, and development.",
								StartDate: days(-30),
								Lectures: []lecture{
									makeLecture("Lecture 1: What is Psychology?", "The scope and history of psychology as a science."),
									makeLecture("Lecture 2: Research Methods", "How psychologists study behavior and the mind."),
								},
								Assignments: []assignment{
									makeAssignment("PSYCH101", "Essay 1: Theories of Development", "Compare two major theories of childhood development."),
									makeAssignment("PSYCH101", "Essay 2: Cognitive Biases", "Describe three cognitive biases with real-world examples."),
								},
								Resources: makeCourseResources("PSYCH101"),
							},
						},
					},
				},
			},
			{
				Name: "Political Science",
				Categories: []category{
					{
						Name: "Year 1",
						Courses: []course{
							{
								ShortName: "POLSCI101",
								FullName:  "Introduction to Political Science",
								Summary:   "Political systems, institutions, and ideologies.",
								StartDate: days(-30),
								Lectures: []lecture{
									makeLecture("Lecture 1: Political Systems", "Comparing democracies, autocracies, and hybrid systems."),
									makeLecture("Lecture 2: Political Ideologies", "Liberalism, conservatism, socialism, and beyond."),
								},
								Assignments: []assignment{
									makeAssignment("POLSCI101", "Essay 1: Compare Two Political Systems", "Compare the political systems of two countries of your choice."),
									makeAssignment("POLSCI101", "Essay 2: Ideology Analysis", "Analyze a political speech for its underlying ideology."),
								},
								Resources: makeCourseResources("POLSCI101"),
							},
						},
					},
				},
			},
			{
				Name: "Communications",
				Categories: []category{
					{
						Name: "Year 1",
						Courses: []course{
							{
								ShortName: "COMM101",
								FullName:  "Introduction to Communications",
								Summary:   "Theories of media, interpersonal communication, and public speaking.",
								StartDate: days(-30),
								Lectures: []lecture{
									makeLecture("Lecture 1: Models of Communication", "Sender, message, channel, receiver, and noise."),
									makeLecture("Lecture 2: Mass Media and Society", "How media shapes and reflects public opinion."),
								},
								Assignments: []assignment{
									makeAssignment("COMM101", "Assignment 1: Media Analysis", "Analyze how a current news story is framed differently across outlets."),
									makeAssignment("COMM101", "Assignment 2: Speech Outline", "Prepare an outline for a five-minute persuasive speech."),
								},
								Resources: makeCourseResources("COMM101"),
							},
						},
					},
				},
			},
			{
				Name: "Economics",
				Categories: []category{
					{
						Name: "Year 1",
						Courses: []course{
							{
								ShortName: "ECON101",
								FullName:  "Principles of Economics",
								Summary:   "Supply and demand, markets, and the basics of macroeconomics.",
								StartDate: days(-30),
								Lectures: []lecture{
									makeLecture("Lecture 1: Supply and Demand", "How prices emerge from market forces."),
									makeLecture("Lecture 2: Markets and Efficiency", "Competition, monopoly, and market failures."),
								},
								Assignments: []assignment{
									makeAssignment("ECON101", "Problem Set 1: Supply and Demand", "Solve the given supply-and-demand problems."),
									makeAssignment("ECON101", "Problem Set 2: Market Structures", "Compare outcomes under competition vs. monopoly."),
								},
								Resources: makeCourseResources("ECON101"),
							},
						},
					},
				},
			},
			{
				Name: "History",
				Categories: []category{
					{
						Name: "Year 1",
						Courses: []course{
							{
								ShortName: "HIST101",
								FullName:  "World History I",
								Summary:   "A survey of world history from antiquity through the early modern period.",
								StartDate: days(-30),
								Lectures: []lecture{
									makeLecture("Lecture 1: Ancient Civilizations", "Mesopotamia, Egypt, and the Indus Valley."),
									makeLecture("Lecture 2: Classical Empires", "Rome, Greece, and their lasting influence."),
								},
								Assignments: []assignment{
									makeAssignment("HIST101", "Essay 1: Compare Two Ancient Civilizations", "Compare the political structures of two ancient civilizations."),
									makeAssignment("HIST101", "Essay 2: Legacy of a Classical Empire", "Discuss the lasting legacy of one classical empire."),
								},
								Resources: makeCourseResources("HIST101"),
							},
						},
					},
				},
			},
		},
		Users: buildUsers(),
	}
}

func buildUsers() []seedUser {
	return []seedUser{
		{
			Username: "alice", Password: "Student123!",
			FirstName: "Alice", LastName: "Nguyen", Email: "alice@example.com",
			Courses: []string{"CS101", "MATH101", "PHYS101"},
		},
		{
			Username: "bob", Password: "Student123!",
			FirstName: "Bob", LastName: "Martinez", Email: "bob@example.com",
			Courses: []string{"CS101", "CS201"},
		},
		{
			Username: "carol", Password: "Student123!",
			FirstName: "Carol", LastName: "Kim", Email: "carol@example.com",
			Courses: []string{"PSYCH101", "POLSCI101", "COMM101"},
		},
		{
			Username: "dave", Password: "Student123!",
			FirstName: "Dave", LastName: "Okafor", Email: "dave@example.com",
			Courses: []string{"ECON101", "HIST101", "MATH101"},
		},
		{
			Username: "eve", Password: "Student123!",
			FirstName: "Eve", LastName: "Rossi", Email: "eve@example.com",
			Courses: []string{"CS201", "MATH201", "PHYS101"},
		},
		{
			Username: "frank", Password: "Student123!",
			FirstName: "Frank", LastName: "Dubois", Email: "frank@example.com",
			Courses: []string{"HIST101", "POLSCI101"},
		},
	}
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func main() {
	s := buildSpec()

	specJSON, err := json.Marshal(s)
	if err != nil {
		fmt.Println("marshal spec:", err)
		os.Exit(1)
	}

	tmpDir, err := os.MkdirTemp("", "moodle-seed")
	if err != nil {
		fmt.Println("mkdir temp:", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	specPath := tmpDir + "/spec.json"
	runnerPath := tmpDir + "/runner.php"

	if err := os.WriteFile(specPath, specJSON, 0o600); err != nil {
		fmt.Println("write spec:", err)
		os.Exit(1)
	}
	if err := os.WriteFile(runnerPath, []byte(runnerPHP), 0o600); err != nil {
		fmt.Println("write runner:", err)
		os.Exit(1)
	}

	fmt.Println("copying spec + runner into container...")
	if err := run("podman", "cp", specPath, containerName+":/bitnami/moodle/seed_spec.json"); err != nil {
		fmt.Println("copy spec:", err)
		os.Exit(1)
	}
	if err := run("podman", "cp", runnerPath, containerName+":/bitnami/moodle/seed_runner.php"); err != nil {
		fmt.Println("copy runner:", err)
		os.Exit(1)
	}

	fmt.Println("running seed inside container...")
	if err := run("podman", "exec", containerName, "php", "/bitnami/moodle/seed_runner.php", "/bitnami/moodle/seed_spec.json"); err != nil {
		fmt.Println("seed run failed:", err)
		os.Exit(1)
	}

	_ = run("podman", "exec", containerName, "rm", "-f", "/bitnami/moodle/seed_runner.php", "/bitnami/moodle/seed_spec.json")
}
