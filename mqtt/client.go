package mqtt

import (
	"context"
	"fmt"
	//"strings"
	"database/sql"
	"log"
	//"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type MQTTClient struct {
	client mqtt.Client
}



func NewMQTTClient(client mqtt.Client) *MQTTClient {
	return &MQTTClient{client: client}
}


func SaveMessageToDB(ctx context.Context, db *sql.DB, payload MessagePayload) error {
    if db == nil {
        return fmt.Errorf("Database connection is nil")
    }

    userName := payload.Username
    cageName := payload.Cagename
    typeName := payload.Type
    dataName := payload.Dataname
    value := payload.Value

    log.Printf("Processing data: User=%s, Cage=%s, Type=%s, Data=%s, Value=%.2f",
        userName, cageName, typeName, dataName, value)

    tx, err := db.BeginTx(ctx, nil)
    if err != nil {
        return fmt.Errorf("Failed to begin transaction: %v", err)
    }
    defer func() {
        if err != nil {
            _ = tx.Rollback()
        }
    }()

    var userID string
    err = tx.QueryRowContext(ctx, `SELECT id FROM public.users WHERE username = $1`, userName).Scan(&userID)
    if err != nil {
        return fmt.Errorf("User not found: %v", err)
    }

    var cageID string
    err = tx.QueryRowContext(ctx, `SELECT id FROM public.cages WHERE name = $1 AND user_id = $2`, cageName, userID).Scan(&cageID)
    if err != nil {
        return fmt.Errorf("Cage '%s' not found for user '%s': %v", cageName, userName, err)
    }

    switch typeName {
    case "sensor":
        _, err = tx.ExecContext(ctx, `
            UPDATE public.sensors
            SET value = $1, updated_at = NOW()
            WHERE cage_id = $2 AND name = $3
        `, value, cageID, dataName)
        if err != nil {
            return fmt.Errorf("Failed to update sensor data: %v", err)
        }

    case "device":
        status := "off"
        if value == 1 {
            status = "on"
        }
        _, err = tx.ExecContext(ctx, `
            UPDATE public.devices
            SET status = $1, updated_at = NOW()
            WHERE cage_id = $2 AND name = $3
        `, status, cageID, dataName)
        if err != nil {
            return fmt.Errorf("Failed to update device data: %v", err)
        }

    default:
        return fmt.Errorf("Unknown type: %s", typeName)
    }

    if err = tx.Commit(); err != nil {
        return fmt.Errorf("Failed to commit transaction: %v", err)
    }

    BroadcastToWebSocket(map[string]interface{}{
        "username": userName,
        "cagename": cageName,
        "type":     typeName,
        "dataname": dataName,
        "value":    value,
    })
    
    log.Println("Data successfully updated in the database")
    return nil
}



// Helper function to determine unit based on sensor type
func determineUnit(sensorType string) string {
    switch sensorType {
    case "temperature":
        return "C"
    case "humidity":
        return "%"
    case "light":
        return "lux"
    case "waterlevel":
        return "mm"
    case "infrared":
        return "yes/no"
    default:
        return ""
    }
}