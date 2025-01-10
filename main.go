package main

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/bwmarrin/discordgo"
)

var MaxWait = 10

func main() {
	err := RunDiscordBot()
	if err != nil {
		log.Fatalf("Error running Discord bot: %v", err)
	}

	log.Println("Bot is now running. Press CTRL-C to exit.")
}
	

func RunDiscordBot() error {
	fmt.Printf("When a new message pops up, you'll have %v seconds to reply. After that, the bot will stop waiting for a response. The timer resets every time you send a keystroke, so it won't cut you off. Press enter when you're done.\n\n\n", MaxWait)

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

    select {}
}


func handleMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
    if m.Author.ID == s.State.User.ID {
        return
    }
	
	fmt.Printf("%v says: %v\n", m.Author.Username, m.ContentWithMentionsReplaced())
	fmt.Print("Reply: ")
	r, err := getResponse(m.Content)
	if err != nil {
		if err.Error() == "No response" {
			fmt.Println("No response received - timed out")
			return
		}
		log.Fatalf("Error getting response: %v", err)
	}

	if r != "" {
		s.ChannelMessageSendReply(m.ChannelID, r, m.Reference())
	}

	fmt.Println("Message sent\n")
}

func getResponse(message string) (string, error) {
    reader := bufio.NewReader(os.Stdin)
    timer := time.NewTimer(time.Duration(MaxWait) * time.Second)

    var input strings.Builder
    for {
        select {
        case <-timer.C:
            return "", errors.New("No response")
        default:
            // Check for input without blocking by using a select with a short timeout
            select {
            case <-time.After(100 * time.Millisecond):
                // No input within 100ms, continue to next iteration
                continue
            default:
                _, _, err := reader.ReadRune()
                if err != nil {
                    if err.Error() == "EOF" {
                        return input.String(), nil
                    }
                    fmt.Println("Error reading input:", err)
                    return "", err
                }
                reader.UnreadRune()

                line, err := reader.ReadString('\n')
                if err != nil {
                    fmt.Println("Error reading input:", err)
                    return "", err
                }
                input.WriteString(line)

                // If we've read a newline, we've got complete input
                if line[len(line)-1] == '\n' {
                    if !timer.Stop() {
                        select {
                        case <-timer.C:
                        default:
                        }
                    }
                    return input.String(), nil
                }
                // Reset timer only when new input is detected
                if !timer.Stop() {
                    <-timer.C // Drain the timer if it has already fired
                }
                timer.Reset(time.Duration(MaxWait) * time.Second)
            }
        }
    }
}
