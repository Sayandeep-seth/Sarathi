
package events

import (
    "encoding/json"
    "log"
)

type BusLocationData struct {
    BusID     string  `json:"bus_id"`
    RouteID   string  `json:"route_id"`
    Latitude  float64 `json:"latitude"`
    Longitude float64 `json:"longitude"`
    Speed     float64 `json:"speed"`
    Heading   float64 `json:"heading"`
}

func HandleBusLocation(payload []byte) error {
    var data BusLocationData

    if err := json.Unmarshal(payload, &data); err != nil {
        return err
    }

    log.Printf(
        "Bus %s location: %f, %f",
        data.BusID,
        data.Latitude,
        data.Longitude,
    )

    return nil
}
