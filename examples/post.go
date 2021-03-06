package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"

	"github.com/joyrexus/buckets"
)

const verbose = false // if `true` you'll see log output

func main() {
	// Open a buckets database.
	bx, err := buckets.Open(tempFilePath())
	if err != nil {
		log.Fatalf("couldn't open db: %v", err)
	}

	// Delete and close the db when done.
	defer os.Remove(bx.Path())
	defer bx.Close()

	// Create a bucket for storing todos.
	todos, err := bx.New([]byte("todos"))
	if err != nil {
		log.Fatalf("couldn't create todos bucket: %v", err)
	}

	// Start our web server.
	handler := service{todos}
	srv := httptest.NewServer(handler)
	defer srv.Close()

	// Daily todos to post.
	posts := map[string]*Todo{
		"/mon": {Day: "mon", Task: "milk cows"},
		"/tue": {Day: "tue", Task: "fold laundry"},
		"/wed": {Day: "wed", Task: "flip burgers"},
		"/thu": {Day: "thu", Task: "join army"},
		"/fri": {Day: "fri", Task: "kill time"},
		"/sat": {Day: "sat", Task: "make merry"},
		"/sun": {Day: "sun", Task: "pray quietly"},
	}

	// Make a series of post requests to our server.
	for path, todo := range posts {
		url := srv.URL + path
		bodyType := "application/json"
		body, err := todo.Encode()
		if err != nil {
			log.Print(err)
		}
		resp, err := http.Post(url, bodyType, body)
		if err != nil {
			log.Print(err)
		}
		if verbose {
			log.Printf("client: %s\n", resp.Status)
		}
	}

	// Test that each encoded todo sent to the server was
	// in fact stored in the todos bucket.
	for route, want := range posts {
		// Get encoded todo sent to route.
		encoded, err := todos.Get([]byte(route))
		if err != nil {
			log.Fatalf("todo bucket is missing entry for %s: %v", route, err)
		}
		got, err := decode(encoded)
		if err != nil {
			log.Fatalf("could not decode entry for %s: %v", route, err)
		}
		if got.Task != want.Task {
			log.Fatalf("%s: got %v, want %v", route, got.Task, want.Task)
		}
		if !reflect.DeepEqual(got, want) {
			log.Fatalf("%s: got %v, want %v", route, got, want)
		}
	}

	// Show the encoded todos now stored in the todos bucket.
	do := func(k, v []byte) error {
		todo, err := decode(v)
		if err != nil {
			log.Print(err)
		}
		fmt.Printf("%s: %s\n", k, todo.Task)
		return nil
	}
	todos.Map(do)

	// Output:
	// /fri: kill time
	// /mon: milk cows
	// /sat: make merry
	// /sun: pray quietly
	// /thu: join army
	// /tue: fold laundry
	// /wed: flip burgers
}

// Todo holds a task description and the day of week in which to do it.
type Todo struct {
	Task string
	Day  string
}

// Encode marshals a Todo into a buffer.
func (todo *Todo) Encode() (*bytes.Buffer, error) {
	b, err := json.Marshal(todo)
	if err != nil {
		return &bytes.Buffer{}, err
	}
	return bytes.NewBuffer(b), nil
}

// This service handles post requests, storing them in a todos bucket.
// The URLs are used as keys and the json-encoded payload as values.
type service struct {
	todos *buckets.Bucket
}

func (s service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	key := []byte(r.URL.String())

	// Read the request body's json payload into a byteslice.
	b, err := ioutil.ReadAll(r.Body)
	todo, err := decode(b)
	if err != nil {
		http.Error(w, err.Error(), 500)
	}

	// Put key/json into todos bucket.
	if err := s.todos.Put(key, b); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	if verbose {
		log.Printf("server: %s: %v", key, todo)
	}

	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintf(w, "put todo for %s: %s\n", key, todo)
}

/* -- UTILITY FUNCTIONS -- */

// decode unmarshals a json-encoded byteslice into a Todo.
func decode(b []byte) (*Todo, error) {
	todo := new(Todo)
	if err := json.Unmarshal(b, todo); err != nil {
		return todo, err
	}
	return todo, nil
}

// tempFilePath returns a temporary file path.
func tempFilePath() string {
	f, _ := ioutil.TempFile("", "bolt-")
	if err := f.Close(); err != nil {
		log.Fatal(err)
	}
	if err := os.Remove(f.Name()); err != nil {
		log.Fatal(err)
	}
	return f.Name()
}
