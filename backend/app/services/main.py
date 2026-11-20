from fastapi import FastAPI

from app.api.v1 import bookings
from app.api.v1 import map_routes
from app.api.v1 import driver
from app.api.v1 import passes

app = FastAPI(title="Sarathi API")

app.include_router(bookings.router, prefix="/api/v1")
app.include_router(map_routes.router, prefix="/api/v1")
app.include_router(driver.router, prefix="/api/v1")
app.include_router(passes.router, prefix="/api/v1")

@app.get("/")
def home():
    return {"message": "Sarathi Backend Running"}