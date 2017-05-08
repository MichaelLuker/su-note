Single Use Note (SU-Note)

SU-Note is a simple to set up server for secure note sharing.
- 1  : Copy a real cert and key into the security folder
- 1.5: Modify line 299 to read the new cert and key if they have different names / locations
- 2  : Run the server 'go run server.go >> su-note.log 2>&1'

Here are some things to note:

- The server might need to be run as root, I wrote it on Windows and haven't tested much in Linux yet
    It needs to be able to bind to ports 80 and 443 to run
- The cert and key files in the security folder are self-signed for localhost, they should be replaced.
- Notes that are more than 30 minutes old will be deleted
- Attempting to unlock a note more than 3 times will destroy the note
- If the server gets restarted the number of attempts to unlock a note will be reset
- Files in the html, img, css, and scripts folder can be modified to customize the looks
    names and id of elements, as wells as items with NOTEURL or NOTEKEY need to exist in their respective 
    files for note creation to work properly