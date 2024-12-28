package main

import (
	"fmt"
	"sync"
	connectionservice "websocket-chat/client/connection-service"
	"websocket-chat/comm"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var (
	message          string
	testChatText     string            = "Juan: Hello, this is a test message\n\nPablo: pablo\n\nGomez: Estan chismeando?\n\nJuan: No, estamos trabajando\n\nGomez: Seguro?\n\nJuan: Si, seguro. Pablo, thoughts?\n\nPablo: Pablo"
	chatMessageInput *tview.InputField = tview.NewInputField()
)

func handleSendMessage(key tcell.Key) {
	fmt.Println("key:", key)
	switch key {
	case tcell.KeyEnter:
		// send message
		connectionservice.SendChat(message)
		chatMessageInput.SetText("")
	}
}

func handleChangeInput(txt string) {
	message = txt
}

func main() {
	var wg sync.WaitGroup
	wg.Add(1)
	messageChannel := make(chan comm.Message)

	go func() {
		defer wg.Done()
		connectionservice.ConnectToChatServer(&messageChannel)
	}()

	app := tview.NewApplication()
	placeholder := tview.NewTextView()

	chatMessageInput.
		SetLabel("Message: ").
		SetFieldWidth(0).
		SetChangedFunc(handleChangeInput).
		SetDoneFunc(handleSendMessage)

	mainView := tview.NewGrid().
		SetRows(0, 3).
		SetBorders(false).
		AddItem(placeholder, 0, 0, 1, 1, 0, 0, false).
		AddItem(chatMessageInput, 1, 0, 1, 1, 0, 0, true)

	fmt.Fprintf(placeholder, "%s", testChatText)

	placeholder.SetBorder(true).SetTitle("Pablo")

	if err := app.SetRoot(mainView, true).EnableMouse(true).Run(); err != nil {
		panic(err)
	}

	// Wait for the goroutine to finish when the user exits the chat
	wg.Wait()
}
