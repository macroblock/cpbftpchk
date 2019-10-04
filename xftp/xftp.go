package xftp

import (
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"syscall"
	"time"

	"golang.org/x/crypto/ssh/terminal"

	"github.com/jlaffaye/ftp"
	"github.com/macroblock/imed/pkg/ptool"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// IFtp -
type IFtp interface {
	//Close()
	FileSize(path string) (int64, error)
	Exists(path string) error
	Delete(path string) error
	Rename(from, to string) error
	StorFrom(path string, r io.Reader, offset uint64) error
	ChangeDir(dir string) error
	CurrentDir() (string, error)
	List(path string) ([]TEntry, error)
	Quit() error
}

// TEntry -
type TEntry struct {
	Name string
	Size int64
	Time time.Time
	Type int
}

// -
const (
	File = iota
	Folder
	Unknown
)

var urlParser *ptool.TParser

func init() {
	//[proto://][username[:password]@]host[/path][:port]
	urlRule := "" +
		"entry = spaces [@proto '://'] [@username[':'@password]'@'] @host ['/'@path] ['/'] [':'@port] spaces$;" +
		"spaces   = {'\x00'..'\x20'};" +
		"anyRune  = !$ '\x00'..'\xff';" +
		"invalid  = '\x00'..'\x20';" +
		"digit    = '0'..'9';" +
		"proto    = {!invalid!':' anyRune};" +
		"username = {!invalid!':'!'@' anyRune};" +
		"password = {!invalid!'@' anyRune};" +
		"host     = {!invalid!':'!'/' anyRune};" +
		"path     = {!invalid!':'!'/:'!('/'$) anyRune};" +
		"port     = digit{digit};"
	p, err := ptool.NewBuilder().FromString(urlRule).Entries("entry").Build()
	if err != nil {
		log.Panic(err, "can't compile url parser")
		panic(err)
	}
	urlParser = p
}

// New -
func New(cs TConnStruct) (IFtp, error) {
	// cs, err := ParseConnString(conn)
	// if err != nil {
	// 	return nil, err
	// }
	addr := cs.Host + ":" + cs.Port
	switch cs.Proto {
	default:
		return nil, fmt.Errorf("unsupported protocol %q", cs.Proto)
	case "ftp":
		conn, err := ftp.DialTimeout(addr, 5*time.Second)
		if err != nil {
			return nil, err
		}
		err = conn.Login(cs.Username, cs.Password)
		if err != nil {
			return nil, err
		}
		c := &TFtp{}
		c.client = conn

		err = c.ChangeDir(cs.Path)
		if err != nil {
			c.Quit()
			return nil, err
		}
		return c, nil
	case "sftp":
		config := &ssh.ClientConfig{
			User: cs.Username,
			Auth: []ssh.AuthMethod{
				ssh.KeyboardInteractive(func(user, instruction string, questions []string, echos []bool) ([]string, error) {
					// Just send the password back for all questions
					answers := make([]string, len(questions))
					for i := range answers {
						answers[i] = cs.Password
					}
					return answers, nil
				}),
				ssh.PasswordCallback(func() (string, error) { return cs.Password, nil }),
				ssh.Password(cs.Password),
			},
			// HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
				return nil
			},
			Timeout: 10 * time.Second,
			// Config: ssh.Config{
			// 	//Ciphers: []string{"aes128-cbc"},
			// 	Ciphers: []string{"3des-cbc", "aes256-cbc", "aes192-cbc", "aes128-cbc"},
			// },
		}
		conn, err := ssh.Dial("tcp", addr, config)
		if err != nil {
			return nil, fmt.Errorf("Failed to dial: " + err.Error())
		}
		client, err := sftp.NewClient(conn)
		if err != nil {
			return nil, fmt.Errorf("Failed to create client: " + err.Error())
		}
		c := &TSftp{}
		c.client = client

		cwd, err := c.client.Getwd()
		if err != nil {
			return nil, err
		}
		c.cwd = cwd
		err = c.ChangeDir(cs.Path)
		if err != nil {
			c.Quit()
			return nil, err
		}
		return c, nil
	}
}

func setDefault(val string, def string) string {
	if val == "" {
		return def
	}
	return val
}

// TConnStruct -
type TConnStruct struct {
	Proto, Username, Password, Host, Path, Port string
}

// ParseConnString -
// conn: user:pswd@proto://host/path:port
// proto - currently ftp or sftp only
// all fields are necessary
func ParseConnString(conn string) (*TConnStruct, error) {
	cs := TConnStruct{}
	tree, err := urlParser.Parse(conn)
	if err != nil {
		return nil, err
	}
	for _, node := range tree.Links {
		name := urlParser.ByID(node.Type)
		ok, err := ptool.SetStructField(&cs, strings.Title(name), node.Value)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("field not found %q", name)
		}
	}
	// fmt.Printf("proto %q\nuser %q\npswd %q\nhost %q\npath %q\nport %q\n", cs.Proto, cs.Username, cs.Password, cs.Host, cs.Path, cs.Port)
	if cs.Port == "" {
		switch cs.Proto {
		case "":
			cs.Proto = "ftp"
			cs.Port = "21"
		case "ftp":
			cs.Port = "21"
		case "sftp":
			cs.Port = "22"
		}
	}

	if cs.Host == "" {
		return nil, fmt.Errorf("host name is empty")
	}

	if cs.Password == "" && cs.Username != "" {
		fmt.Print("Enter Password: ")
		bytePassword, err := terminal.ReadPassword(int(syscall.Stdin))
		if err != nil {
			return nil, err
		}
		// fmt.Println("\nPassword typed: " + string(bytePassword))
		cs.Password = string(bytePassword)
	}
	return &cs, nil
}
