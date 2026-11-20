from fastapi import APIRouter

router = APIRouter()

@router.post("/bookings")
async def create_booking():
    return {
        "message": "Booking API working"
    }