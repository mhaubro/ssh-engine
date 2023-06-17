package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/spf13/viper"
	"golang.org/x/crypto/ssh"
)

func main() {
	// Read configuration
	configuration := readConfiguration()
	debugLogging := false

	// Setup logging if a log file name was passed in
	if configuration.LogFileName != "" {
		file, err := os.OpenFile("engine.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()
		log.SetOutput(file)

		debugLogging = true
	}

	server := fmt.Sprintf("%s:%s", configuration.Host, configuration.Port)

	// Setup the client configuration
	sshConfig, err := getSshConfig(configuration)
	if err != nil {
		log.Fatalf("Failed to get SSH configuration: %s", err)
	}

	// Start the connection
	client, err := ssh.Dial("tcp", server, sshConfig)
	if err != nil {
		log.Fatalf("Could not connect to SSH (failed to dial): %s", err)
	}
	defer client.Close()

	// Start a session
	session, err := client.NewSession()
	if err != nil {
		log.Fatalf("Failed to create SSH session: %s", err)
	}
	defer session.Close()

	session.Stdout = os.Stdout
	session.Stderr = os.Stderr

	// StdinPipe for commands
	stdin, _ := session.StdinPipe()

	// Start remote shell
	if err := session.Shell(); err != nil {
		log.Fatalf("Failed to start shell: %s", err)
	}

	// Run the supplied command first
	fmt.Fprintf(stdin, "%s\n", configuration.RemoteCommand)

	// Accepting commands
	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		input := scanner.Text()

		if debugLogging {
			log.Println("Input: " + input)
		}

		fmt.Fprintf(stdin, "%s\n", input)
		if input == "quit" {
			if debugLogging {
				log.Println("Quit sent")
			}
			break
		}
	}
}

func getSshConfig(configuration Configurations) (*ssh.ClientConfig, error) {
	key, err := getKeyFile(configuration.PrivateKeyFile)
	if err != nil {
		return nil, fmt.Errorf("could not read privateKeyFile at %s: %w", configuration.PrivateKeyFile, err)
	}

	sshConfig := &ssh.ClientConfig{
		User: configuration.User,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(key),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	return sshConfig, nil
}

func getKeyFile(file string) (ssh.Signer, error) {
	buf, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("error reading the key file: %w", err)
	}

	key, err := ssh.ParsePrivateKey(buf)
	if err != nil {
		return nil, fmt.Errorf("error parsing the private key file. Is this a valid private key?: %w", err)
	}

	return key, nil
}

func readConfiguration() Configurations {
	if _, err := os.Stat("engine.yml"); os.IsNotExist(err) {
		fmt.Println("The file 'engine.yml' could not be found in the current directory")
		os.Exit(1)
	}

	viper.SetConfigName("engine")
	viper.SetConfigType("yml")
	viper.AddConfigPath(".")

	// Read the configuration
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			fmt.Println("No such config file")
		} else {
			fmt.Printf("Error reading the engine.yml file: %s", err)
		}
		os.Exit(1)
	}

	var configuration Configurations
	if err := viper.Unmarshal(&configuration); err != nil {
		fmt.Printf("Unable to decode the engine.yml file: %v", err)
		os.Exit(1)
	}

	return configuration
}

type Configurations struct {
	User           string `mapstructure:"user"`
	PrivateKeyFile string `mapstructure:"privateKeyFile"`
	Host           string `mapstructure:"host"`
	Port           string `mapstructure:"port"`
	RemoteCommand  string `mapstructure:"remoteCommand"`
	LogFileName    string `mapstructure:"logFileName"`
}
