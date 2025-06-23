from fastapi import FastAPI, Request
from fastapi.responses import HTMLResponse, JSONResponse
from fastapi.templating import Jinja2Templates
from fastapi.staticfiles import StaticFiles
from fastapi.middleware.cors import CORSMiddleware
import httpx
import os
import logging

# Set up logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

# Global configuration
DEFRADB_GRAPHQL_PLAYGROUND_URL = "http://127.0.0.1:9181/api/v0/graphql"
API_ORIGIN = "http://127.0.0.1:8000"

app = FastAPI()

# Configure CORS
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],  # Allows all origins in development
    allow_credentials=True,
    allow_methods=["GET", "POST", "OPTIONS"],
    allow_headers=["*"],
    expose_headers=["*"]
)

# Get the absolute path to the web directory
web_dir = os.path.dirname(os.path.abspath(__file__))
templates = Jinja2Templates(directory=web_dir)

@app.options("/{path:path}")
async def options_route(request: Request):
    return JSONResponse(
        content={},
        headers={
            "Access-Control-Allow-Origin": "*",
            "Access-Control-Allow-Methods": "GET, POST, OPTIONS",
            "Access-Control-Allow-Headers": "*"
        }
    )

async def fetch_graphql_data(client, query):
    """Helper function to fetch data from DefraDB"""
    try:
        response = await client.post(
            DEFRADB_GRAPHQL_PLAYGROUND_URL,
            json={"query": query},
            headers={
                "Content-Type": "application/json",
                "Accept": "application/json",
                "Origin": API_ORIGIN
            }
        )
        logger.info(f"GraphQL response status: {response.status_code}")
        if response.status_code == 200:
            json_data = response.json()
            logger.info(f"GraphQL response data: {json_data}")
            return json_data
        logger.error(f"GraphQL error response: {response.text}")
        return None
    except Exception as e:
        logger.error(f"Error fetching data: {str(e)}")
        return None

@app.get("/", response_class=HTMLResponse)
async def home(request: Request):
    data = {
        "blocks": [],
        "transactions": [],
        "logs": [],
        "events": []
    }
    error = None

    try:
        async with httpx.AsyncClient(timeout=30.0) as client:
            # Query blocks with their transactions
            blocks_query = """
            query {
                Block(limit: 10) {
                    hash
                    number
                    time
                    gasUsed
                    gasLimit
                    size
                    transactions {
                        hash
                        blockNumber
                        from
                        to
                        value
                        gas
                    }
                }
            }
            """
            logger.info("Fetching blocks from DefraDB...")
            blocks_data = await fetch_graphql_data(client, blocks_query)
            
            if blocks_data and "data" in blocks_data:
                blocks = blocks_data["data"].get("Block", [])
                # Sort blocks by number in descending order
                blocks.sort(key=lambda b: int(b["number"], 16) if b["number"].startswith("0x") else int(b["number"]), reverse=True)
                data["blocks"] = blocks

                # Extract transactions from blocks
                transactions = []
                for block in blocks:
                    if block.get("transactions"):
                        transactions.extend(block["transactions"])
                data["transactions"] = transactions

                logger.info(f"Found {len(blocks)} blocks and {len(transactions)} transactions")
            else:
                if blocks_data and "errors" in blocks_data:
                    error = f"GraphQL Error: {blocks_data['errors']}"
                    logger.error(f"GraphQL error: {blocks_data['errors']}")
                else:
                    error = "Failed to fetch data from DefraDB"
                    logger.error("No data returned from DefraDB")

    except httpx.TimeoutException:
        error = "Request to DefraDB timed out. The service might be busy."
        logger.exception("Timeout error fetching data from DefraDB")
    except Exception as e:
        error = f"Failed to fetch data from DefraDB: {str(e)}"
        logger.exception("Error fetching data from DefraDB")
        
    response = templates.TemplateResponse(
        "blocks.html",
        {
            "request": request,
            "error": error,
            **data
        }
    )
    # Add CORS headers to the response
    response.headers["Access-Control-Allow-Origin"] = "*"
    response.headers["Access-Control-Allow-Methods"] = "GET, POST, OPTIONS"
    response.headers["Access-Control-Allow-Headers"] = "*"
    return response
