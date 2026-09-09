package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

const site = "http://localhost:8080"

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

	token, err := Login(site, username, password)
	if err != nil {
		fmt.Println("User does not exist or invalid credentials.")
		os.Exit(1)
	}

	client := New(site, token)

	u, err := client.GetUserByUsername(username)
	if err != nil {
		fmt.Println("lookup failed:", err)
		os.Exit(1)
	}
	if u == nil {
		fmt.Println("User does not exist.")
		os.Exit(1)
	}

	fmt.Println("\nUser found:")
	fmt.Printf("  id:         %d\n", u.ID)
	fmt.Printf("  username:   %s\n", u.Username)
	fmt.Printf("  full name:  %s\n", u.FullName)
	fmt.Printf("  email:      %s\n", u.Email)
	fmt.Printf("  department: %s\n", u.Department)
	fmt.Printf("  suspended:  %t\n", u.Suspended)
	fmt.Printf("  confirmed:  %t\n", u.Confirmed)

	courses, err := client.GetUserCourses(u.ID)
	if err != nil {
		fmt.Println("courses lookup failed:", err)
		os.Exit(1)
	}

	fmt.Printf("  courses:    %d\n", len(courses))
}
