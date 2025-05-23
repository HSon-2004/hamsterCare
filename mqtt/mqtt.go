package mqtt

import (
	"fmt"
	"os"
	"log"
	"os/signal"
	"syscall"
	"database/sql"
	"time"
	"encoding/json"
	"github.com/eclipse/paho.mqtt.golang"
	"net/http"

	"github.com/gorilla/websocket"
)


var upgrader = websocket.Upgrader{
    CheckOrigin: func(r *http.Request) bool {
        return true // Allow all origins (adjust for production)
    },
}

var clients = make(map[*websocket.Conn]bool) // Connected clients
var broadcast = make(chan map[string]interface{}) // Broadcast channel

// WebSocketHandler handles WebSocket connections
func WebSocketHandler(w http.ResponseWriter, r *http.Request) {
    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        log.Printf("WebSocket upgrade error: %v", err)
        return
    }
    defer conn.Close()

    clients[conn] = true
    log.Println("New WebSocket client connected")

    for {
        _, _, err := conn.ReadMessage()
        if err != nil {
            log.Printf("WebSocket read error: %v", err)
            delete(clients, conn)
            break
        }
    }
}

// BroadcastToWebSocket sends data to all connected WebSocket clients
func BroadcastToWebSocket(data map[string]interface{}) {
    broadcast <- data
}

// StartWebSocketServer starts the WebSocket server
func StartWebSocketServer(port string) {
	log.Printf("WebSocket server started on port %s", port)
    http.HandleFunc("/ws", WebSocketHandler)

    go func() {
        for {
            message := <-broadcast
            for client := range clients {
                err := client.WriteJSON(message)
                if err != nil {
                    log.Printf("WebSocket write error: %v", err)
                    client.Close()
                    delete(clients, client)
                }
            }
        }
    }()

    log.Printf("WebSocket server started on port %s", port)
    log.Fatal(http.ListenAndServe(":"+port, nil))
}

type TopicConfig struct {
	Temperature string
	Humidity    string
	Light       string
	WaterLevel  string
	Fan		 	string
	LED 	 	string
	Pump		string
}

func DefaultTopics() TopicConfig {
	return TopicConfig{
		Temperature: "sensor/1/temperature",
		Humidity:    "sensor/2/humidity",
		Light:       "sensor/3/light",
		WaterLevel:  "sensor/4/water-level",
		Fan:         "device/6/fan",
		LED:         "device/7/led",
		Pump:        "device/8/pump",
	}
}

func (tc *TopicConfig) GetAllTopics() []string {
	return []string{
		tc.Temperature,
		tc.Humidity,
		tc.Light,
		tc.WaterLevel,
		tc.Fan,
		tc.LED,
		tc.Pump,
	}
}

func StartMQTTClientSub(db *sql.DB, broker string) {
	topic := DefaultTopics()

	opts := mqtt.NewClientOptions()
	opts.AddBroker(broker)
	opts.SetClientID("go_mqtt_sub")
    opts.SetUsername("user@123")  // Thay bằng username thật
    opts.SetPassword("user@123")  // Thay bằng password thật
	opts.OnConnect = func(client mqtt.Client) {
		fmt.Println("Connected to MQTT broker")
		
		for _, topic := range topic.GetAllTopics() {
			
			topic = "hamster/user1/cage1/" + topic 
			if token := client.Subscribe(topic, 1, MqttHandler(db)); token.Wait() && token.Error() != nil {
                fmt.Printf("Error subscribing to topic %s: %v\n", topic, token.Error())
            } else {
                fmt.Printf("Subscribed to topic: %s\n", topic)
            }
		}
	}
	opts.OnConnectionLost = func(client mqtt.Client, err error) {
		fmt.Println("Connection lost: ", err)
	}
	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Fatal("Error connecting to MQTT broker: ", token.Error())
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Println("Disconnecting from MQTT broker")
	client.Disconnect(250)
	fmt.Println("Disconnected from MQTT broker")
}

func StartMQTTClientPub(broker string, topic string, value int, typename string, id int, dataname string) {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(broker)
	opts.SetClientID("go_mqtt_pub")
	opts.SetUsername("user@123")  // Thay bằng username thật
	opts.SetPassword("user@123")  // Thay bằng password thật
	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Fatal("Error connecting to MQTT broker: ", token.Error())
	}
	//defer client.Disconnect(250)

	payload := map[string]interface{}{
        "username": "user1",	
        "cagename": "cage1",
        "type":     typename,
        "id":       id,
        "dataname": dataname,
        "value":    value,
        "time":     time.Now().UnixNano() / int64(time.Millisecond),
    }

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		log.Fatal("Error marshalling JSON: ", err)
	}

	if token := client.Publish(topic, 0, false, jsonPayload); token.Wait() && token.Error() != nil {
		log.Fatal("Error publishing message: ", token.Error())
	} else {
		fmt.Printf("Published message to topic %s: %s\n", topic, jsonPayload)
	}
}
