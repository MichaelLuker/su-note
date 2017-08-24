// Random generation functions from https://siongui.github.io/2015/04/13/go-generate-random-string/
// HTTP and HTTPS server and setup from https://gist.github.com/d-schmidt/587ceec34ce1334a5e60
// Encryption and decryption functions from https://gist.github.com/manishtpatel/8222606

package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha512"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	mrand "math/rand"

	crand "crypto/rand"
)

var rnd *mrand.Rand

var noteList map[string]int

//const keyChars = "|3@ebwHSl$2~st5!Go91=I[YuxQ#&8]\\VnW*aXU{jkAT+`^g}zRZLJNFEdmrPf6h_Dy(ivq7C4%MB-0cp)KO"
const keyChars = "3ebwHSl2st5Go91IYuxQ8VnWaXUjkATgzRZLJNFEdmrPf6hDyivq7C4MB0cpKO"
const urlChars = "SwsZfeuUj8rG0KxAP3aFhLi2tVlm1TEgoIzHBJvWOk7CybqXdD9nQYR56cMN4p"

// Redirect 80 to 443, force to /
func tlsRedirect(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "https://"+r.Host, http.StatusPermanentRedirect)
}

// Function to generate a random key
func generateKey() []byte {
	result := make([]byte, 32)
	for i := range result {
		result[i] = keyChars[rnd.Intn(len(keyChars))]
	}
	return result
}

// Function to generate a random url string
func generateNoteURL() string {
	result := make([]byte, 32)
	for i := range result {
		result[i] = urlChars[rnd.Intn(len(urlChars))]
	}
	return string(result)
}

// Function to encrypt text
func encrypt(key []byte, text string) string {
	plaintext := []byte(text)

	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err)
	}

	// The IV needs to be unique, but not secure. Therefore it's common to
	// include it at the beginning of the ciphertext.
	ciphertext := make([]byte, aes.BlockSize+len(plaintext))
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(crand.Reader, iv); err != nil {
		panic(err)
	}

	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], plaintext)

	// convert to base64
	return base64.URLEncoding.EncodeToString(ciphertext)
}

// Function to decrypt text
func decrypt(key []byte, cryptoText string) string {
	ciphertext, _ := base64.URLEncoding.DecodeString(cryptoText)

	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err)
	}

	// The IV needs to be unique, but not secure. Therefore it's common to
	// include it at the beginning of the ciphertext.
	if len(ciphertext) < aes.BlockSize {
		panic("ciphertext too short")
	}
	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]

	stream := cipher.NewCFBDecrypter(block, iv)

	// XORKeyStream can work in-place if the two arguments are the same.
	stream.XORKeyStream(ciphertext, ciphertext)

	return fmt.Sprintf("%s", ciphertext)
}

// Function to create a note
func createNote(r *http.Request) string {
	// Prep inputs from the request
	r.ParseForm()

	// Generate a key
	key := generateKey()

	// Encrypt the note
	cryptoText := encrypt(key, r.PostFormValue("noteContent"))

	// Generate an ID
	fileURL := generateNoteURL()

	// Make sure the ID isn't already in use
	if _, err := os.Stat("note/" + fileURL); err == nil {
		for err == nil {
			fileURL = generateNoteURL()
			_, err = os.Stat("note/" + fileURL)
		}
	}

	// Generate note page
	notePage, _ := ioutil.ReadFile("html/noteTemplate.html")

	substr := strings.Replace(string(notePage), "NOTEURL", fileURL, 1)
	ioutil.WriteFile("note/"+fileURL, []byte(substr), 0600)

	// Store the encrypted note, use a hash of the key to help with checking later
	hasher := sha512.New()
	hasher.Write(key)
	kh := base64.URLEncoding.EncodeToString(hasher.Sum(nil))
	ioutil.WriteFile("data/"+fileURL+kh, []byte(cryptoText), 0600)

	// Update the list of accessible notes
	noteList[fileURL] = 0

	// Write out the URL and key
	successPage, _ := ioutil.ReadFile("html/successTemplate.html")
	substr = strings.Replace(string(successPage), "NOTEURL", "https://"+r.Host+"/note/"+fileURL, 1)
	substr = strings.Replace(substr, "NOTEKEY", string(key), 1)

	log.Print("Created note : " + fileURL)
	return substr

}

// Function to attempt unlocking a note
func unlockNote(r *http.Request) string {
	// Prep inputs from the request
	r.ParseForm()

	// Grab the key and ID that were sent
	key := r.PostFormValue("key")
	noteID := r.PostFormValue("noteID")

	// Try to unlock the note
	hasher := sha512.New()
	hasher.Write([]byte(key))
	kh := base64.URLEncoding.EncodeToString(hasher.Sum(nil))
	noteData, err := ioutil.ReadFile("data/" + noteID + kh)
	if err != nil {
		// If the file doesn't exist add an attempt to the note in the list
		log.Print(r.RemoteAddr + " : Used incorrect password on note " + noteID)
		noteList[noteID]++
		reapNotes()
		return "Error: that key doesn't match this note."
	}

	// If the key hash and note id match up to a file, then it can be decrypted and returned
	res := decrypt([]byte(key), string(noteData))

	// Delete the note now that it's been unlocked
	deleteNote(noteID)

	// Send the contents back to the requesting page
	return res
}

// Function to handle note reaping from age or attempts
func reapNotes() {
	// Check for any attempts >= 3
	for note := range noteList {
		if noteList[note] >= 3 {
			log.Print("Too many failed attempts on : " + note)
			deleteNote(note)
		}
	}
	// Check for any notes that are >= 30 minutes old
	files, _ := ioutil.ReadDir("note")
	for _, file := range files {
		// Ignore the file used to keep the directory in git
		if file.Name() != ".keep" {
			// Check the time since the file was created
			if time.Since(file.ModTime()).Minutes() >= 30 {
				log.Print("Reaping old note : " + file.Name())
				deleteNote(file.Name())
			}
		}
	}
}

// Function to remove a note and its bit 'n bobs
func deleteNote(noteID string) {
	// Remove it from the noteList
	delete(noteList, noteID)

	// Delete the note data without needing the key hash
	files, _ := ioutil.ReadDir("data")
	for _, file := range files {
		// If the file contains the ID which should be unique, delete it
		if strings.Contains(file.Name(), noteID) {
			os.Remove("data/" + file.Name())
		}
	}

	// Delete the html page
	os.Remove("note/" + noteID)

	log.Print("Deleted note : " + noteID)
}

// Function to handle a request
func handleRequest(w http.ResponseWriter, r *http.Request) {
	action := strings.Split(r.RequestURI, "/")

	// Switch on the request
	switch action[1] {
	case "createNote":
		io.WriteString(w, createNote(r))
		break
	case "unlockNote":
		io.WriteString(w, unlockNote(r))
		break
	// Serve up static files
	case "css":
		fallthrough
	case "img":
		fallthrough
	case "scripts":
		fallthrough
	case "note":
		// Make sure the file exists
		if _, err := os.Stat(r.RequestURI[1:]); err != nil {
			log.Print("Resource does not exist " + r.RequestURI)
			// Present the error page if it doesn't
			returnDoc, _ := ioutil.ReadFile("html/errorPage.html")
			io.WriteString(w, string(returnDoc))
		} else {
			// Otherwise return the file
			returnDoc, _ := ioutil.ReadFile(r.RequestURI[1:])
			io.WriteString(w, string(returnDoc))
		}
		break
	// Home page
	case "":
		fallthrough
	case "home":
		returnDoc, _ := ioutil.ReadFile("html/homePage.html")
		io.WriteString(w, string(returnDoc))
		break
	default:
		// Present the error page if it's an unknown action
		log.Print(r.RemoteAddr + " : Requested an invalid resource " + r.RequestURI)
		returnDoc, _ := ioutil.ReadFile("html/errorPage.html")
		io.WriteString(w, string(returnDoc))
		break
	}
}

// Start the servers
func main() {
	// Initialize rand
	rnd = mrand.New(mrand.NewSource(time.Now().UnixNano()))

	// Look for any current notes
	noteList = map[string]int{}
	files, _ := ioutil.ReadDir("note")
	for _, file := range files {
		if file.Name() != ".keep" {
			noteList[file.Name()] = 0
		}
	}

	// Get rid of any old notes
	reapNotes()

	// HTTP server
	go http.ListenAndServe(":80", http.HandlerFunc(tlsRedirect))

	// HTTPS server
	mux := http.NewServeMux()
	mux.HandleFunc("/", handleRequest)
	http.ListenAndServeTLS(":443", "security/cert.pem", "security/key.pem", mux)
}
