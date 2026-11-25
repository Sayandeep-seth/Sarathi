package routes

import (
    "net/http"

    sarathiws "your-module-name/internal/websocket"
)

func RegisterWebSocketRoutes(
    mux *http.ServeMux,
    hub *sarathiws.Hub,
) {
    mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {

        // TEMPORARY DEVELOPMENT IDENTITY
        // Replace this with real JWT authentication.
        userID := r.URL.Query().Get("user_id")
        role := r.URL.Query().Get("role")

        if userID == "" || role == "" {
            http.Error(
                w,
                "Missing user identity",
                http.StatusUnauthorized,
            )
            return
        }

        sarathiws.ServeWS(hub, userID, role, w, r)
    })
}