package main

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"os"
        "sync"
	"time"

	"github.com/joho/godotenv"
	"github.com/bwmarrin/discordgo"
)

var (
    MaxReply = 10
    Queue = MessageQueue{}
    inputQueue = make(chan string, 100)
)

func init() {
    go func () {
        reader := bufio.NewReader(os.Stdin)
        for {
            line, err := reader.ReadString('\n')
            if err == nil {
                inputQueue <- line
            }
        }
    }()
}

type SecondsTimer struct {
    timer *time.Timer
    end   time.Time
}

func newSecondsTimer(d time.Duration) *SecondsTimer {
    return &SecondsTimer{
        timer: time.NewTimer(d),
        end:   time.Now().Add(d),
    }
}

func (s *SecondsTimer) Reset(d time.Duration) {
    s.timer.Reset(d)
    s.end = time.Now().Add(d)
}

func (s *SecondsTimer) Stop() bool {
    return s.timer.Stop()
}

func (s *SecondsTimer) TimeRemaining() time.Duration {
    return s.end.Sub(time.Now())
}

type QueuedMessage struct {
    Message   *discordgo.MessageCreate
    Session   *discordgo.Session
}

type MessageQueue struct {
    messages []QueuedMessage
    mu       sync.Mutex
}

func (q *MessageQueue) Enqueue(message QueuedMessage) {
    q.mu.Lock()
    defer q.mu.Unlock()
    q.messages = append(q.messages, message)
}

func (q *MessageQueue) Dequeue() (QueuedMessage, bool) {
    q.mu.Lock()
    defer q.mu.Unlock()

    if len(q.messages) == 0 {
        return QueuedMessage{}, false
    }

    message := q.messages[0]
    q.messages = q.messages[1:]
    return message, true
}

func (q *MessageQueue) ProcessMessages() {
    for {
        queuedMessage, ok := q.Dequeue()
        if !ok {
            time.Sleep(1 * time.Second) // No messages in queue, sleep for a while
            continue
        }

        err := procMessage(queuedMessage.Message, queuedMessage.Session)
        if err != nil {
            q.Enqueue(queuedMessage) // Requeue the message if failed
        }

        time.Sleep(1 * time.Second) // Try to keep from sending messages toooo quickly
    }
}

func main() {
    err := RunDiscordBot()
    if err != nil {
        log.Fatalf("Error running Discord bot: %v", err)
    }

    log.Println("Bot is now running. Press CTRL-C to exit.")
}

func RunDiscordBot() error {
    fmt.Printf("When a new message pops up, you'll have %v seconds to reply. After that, the bot will stop waiting for a response and move on to the next message (or wait for another).", MaxReply)
    fmt.Printf("The timer resets every time you send a keystroke, so it won't cut you off. Press enter when you're done.\n")
    fmt.Printf("If you type when you're not supposed to, things will get messed up. Don't do that.\n")
    fmt.Printf("If you want to quit, press CTRL-C.\n\n")

    err := godotenv.Load()
    if err != nil {
            log.Fatalf("Error loading .env file")
    }

    t := ""
    t = os.Getenv("DISCORD_TOKEN")
    if t == "" {
            log.Fatalf("No token provided. Please set DISCORD_TOKEN environment variable.")
    }

    dg, err := discordgo.New("Bot " + t)
    if err != nil {
        log.Fatalf("Error creating Discord session: %v", err)
    }

    dg.AddHandler(handleMessageCreate)

    err = dg.Open()
    if err != nil {
        log.Fatalf("Error opening Discord connection: %v", err)
    }

    go Queue.ProcessMessages()

    select {}
}

func handleMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
    if m.Author.ID == s.State.User.ID {
        return
    }
	
    Queue.Enqueue(QueuedMessage{Message: m, Session: s})
}

func procMessage(m *discordgo.MessageCreate, s *discordgo.Session) error {
    fmt.Printf("%v says: %v\n", m.Author.Username, m.ContentWithMentionsReplaced())
    fmt.Print("Reply: ")
    r, err := getResponse()
    if err != nil {
        if err.Error() == "No response" {
            fmt.Println("No response received - timed out\n")
            return nil
        }
        log.Fatalf("Error getting response: %v", err)
    }

    if r != "" {
        s.ChannelMessageSendReply(m.ChannelID, r, m.Reference())
        fmt.Println("Message sent\n")
    } else {
        fmt.Println("Message skipped\n")
    }

    return nil
}

func getResponse() (string, error) {
    timer := time.NewTimer(time.Duration(MaxReply) * time.Second)
    defer timer.Stop()

    select {
    case input := <-inputQueue:
        return input, nil
    case <-timer.C:
        return "", errors.New("No response")
    }
}
