from fastapi import APIRouter

router = APIRouter()

@router.get("/map/routes")
async def get_routes(
    origin: str,
    destination: str,
    car_model: str
):
    return {
        "origin": origin,
        "destination": destination,
        "car_model": car_model,
        "message": "Route API working"
    }