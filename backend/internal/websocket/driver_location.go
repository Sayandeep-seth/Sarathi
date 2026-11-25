func (c *Client) handleDriverLocation(message Message) {
    // Check role.
    if c.Role != "driver" {
        return
    }

    if message.RideID == "" {
        return
    }

    // Verify that this driver is assigned to this ride
    // through your booking/dispatch authorization service.
    //
    // Do NOT trust the RideID sent by the client.

    // Decode GPS data.
    // Validate coordinates and speed.
    // Update Redis.
    // Publish Redis Pub/Sub event.
}