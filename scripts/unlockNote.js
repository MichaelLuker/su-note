function unlockNote() {
    // Send the ID and given key
    $.post("/unlockNote", {
            "noteID": document.getElementById("noteID").value,
            "key": document.getElementById("key").value
        },
        function(data,status){
            // Write the result to the text area
            document.getElementById("noteContent").value = data;
        }
    );
}