from fastapi import APIRouter

router = APIRouter()

@router.post("/passes/validate")
async def validate_pass():
    return {
        "message": "Pass validation API working"
    }