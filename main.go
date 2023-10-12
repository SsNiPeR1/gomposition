package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
)

type Packet struct {
	PacketLength    byte
	RequestType     byte  // 0x00 for status request
	ProtocolVersion int16 // Corrected to int16 to accommodate VarInt
	HostnameLength  byte
	Hostname        []byte
	Port            uint16
	NextState       byte
}

type ResponsePacket struct {
	PacketID       byte
	ResponseLength []byte
	Response       []byte
}

type StatusResponse struct {
	Version            StatusVersion     `json:"version"`
	Players            StatusPlayers     `json:"players"`
	Description        StatusDescription `json:"description"`
	Favicon            string            `json:"favicon"`
	EnforcesSecureChat bool              `json:"enforcesSecureChat,omitempty"`
	PreviewsChat       bool              `json:"previewsChat,omitempty"`
}

type StatusVersion struct {
	Name     string `json:"name"`
	Protocol int    `json:"protocol"`
}

type StatusPlayers struct {
	Max    int                   `json:"max"`
	Online int                   `json:"online"`
	Sample []StatusPlayersSample `json:"sample"`
}

type StatusPlayersSample struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

type StatusDescription struct {
	Text string `json:"text"`
}

func main() {
	// Replace these with your server's IP and port
	serverIP := "0.0.0.0"
	serverPort := 25566

	// Create a listener for incoming connections
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", serverIP, serverPort))
	if err != nil {
		fmt.Println("Error creating listener:", err)
		os.Exit(1)
	}
	defer listener.Close()
	fmt.Println("Minecraft server listening on", listener.Addr())

	for {
		// Accept incoming connections
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Error accepting connection:", err)
			continue
		}

		// Handle the connection in a goroutine
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	// Read the packet from the client
	buffer := make([]byte, 1024)
	n, err := conn.Read(buffer) // n is the number of bytes read

	if err != nil {
		fmt.Println("Error reading packet:", err)
		return
	}

	// Parse the packet
	packet := Packet{}

	if int(buffer[0]) != (n - 1) {
		return // Old ping packet, without packet length, ignore it
	}

	packet.PacketLength = buffer[0]
	packet.RequestType = buffer[1]
	packet.ProtocolVersion, _ = readVarInt(buffer[2:]) // Two bytes for protocol version
	packet.HostnameLength = buffer[4]
	packet.Hostname = buffer[5 : 5+packet.HostnameLength]
	packet.Port = uint16(buffer[5+packet.HostnameLength])<<8 | uint16(buffer[5+packet.HostnameLength+1])
	packet.NextState = buffer[5+packet.HostnameLength+2]

	switch packet.NextState {
	case 1: // status
		buffer := make([]byte, 1024) // status request
		conn.Read(buffer)
		jsonResponse, _ := json.Marshal(StatusResponse{
			Version: StatusVersion{
				Name:     "Gomposition 1.20.1",
				Protocol: 763,
			},
			EnforcesSecureChat: true,
			Description: StatusDescription{
				Text: "Hello from Gomposition!",
			},
			Players: StatusPlayers{
				Max:    20,
				Online: 0,
				Sample: []StatusPlayersSample{},
			},
			PreviewsChat: true,
		})
		l := makeVarInt(len(jsonResponse))
		pkt := ResponsePacket{
			PacketID:       0x00,
			ResponseLength: l,
			Response:       jsonResponse,
		}
		responseBuffer := []byte{0x00, 0x01, pkt.PacketID}
		responseBuffer = append(responseBuffer, pkt.ResponseLength...)
		responseBuffer = append(responseBuffer, pkt.Response...)
		responseBuffer[0] = byte(len(responseBuffer) - 2) // -2 for 0x00 and 0x01
		_, err := conn.Write(responseBuffer)
		if err != nil {
			fmt.Println("Error writing response:", err)
			return
		}
	}

	buf1 := make([]byte, 1024)
	n, _ = conn.Read(buf1)

	if buf1[1] == 0x01 {
		// ping pong (return the same long from the request)
		responseBuffer := []byte{0x00, 0x01}
		// append the value from the request
		responseBuffer = append(responseBuffer, buf1[2:n]...)
		// first byte is packet length
		responseBuffer[0] = byte(len(responseBuffer) - 1)

		_, err := conn.Write(responseBuffer)
		if err != nil {
			fmt.Println("Error writing response:", err)
			return
		}
	}
}

func readVarInt(data []byte) (int16, int) {
	var result int16
	var shift uint
	var bytesRead int

	for {
		b := data[bytesRead]
		result |= (int16(b&0x7F) << shift)
		shift += 7
		bytesRead++

		if bytesRead > 5 {
			break // Prevent potential infinite loop
		}

		if b&0x80 == 0 {
			break
		}
	}

	return result, bytesRead
}

func makeVarInt(value int) []byte {
	var result []byte

	for {
		temp := value & 0x7F
		value >>= 7

		if value != 0 {
			temp |= 0x80
		}

		result = append(result, byte(temp))

		if value == 0 {
			break
		}
	}

	return result
}
