from fastapi import APIRouter

router = APIRouter()

@router.post("/driver/location")
async def update_driver_location():
    return {
        "message": "Driver location updated"
    }