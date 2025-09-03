# Chat History API Documentation

This document describes the chat history API endpoints for retrieving message history between users and in groups.

## Authentication

All endpoints require authentication. Include the JWT token in the Authorization header:

```
Authorization: Bearer <your_jwt_token>
```

## Endpoints

### 1. Get Chat History by Conversation ID

Retrieve chat history for a specific conversation.

**Endpoint:** `GET /chat/history/:conversation_id`

**Parameters:**

- `conversation_id` (path): The UUID of the conversation
- `limit` (query, optional): Number of messages to return (default: 50, max: 100)
- `offset` (query, optional): Number of messages to skip (default: 0)

**Response:**

```json
{
  "conversation_id": "uuid",
  "messages": [
    {
      "id": "message_uuid",
      "sender_id": "user_uuid",
      "content": "Hello, how are you?",
      "message_type": "text",
      "message_status": "seen",
      "timestamp": 1640995200,
      "conversation_id": "conversation_uuid"
    }
  ],
  "limit": 50,
  "offset": 0,
  "total": 1
}
```

### 2. Get Chat History with Specific User

Retrieve chat history between the authenticated user and another specific user.

**Endpoint:** `GET /chat/history/with/:user_id`

**Parameters:**

- `user_id` (path): The UUID of the other user
- `limit` (query, optional): Number of messages to return (default: 50, max: 100)
- `offset` (query, optional): Number of messages to skip (default: 0)

**Response:**

```json
{
  "conversation_id": "uuid",
  "conversation_type": "private",
  "other_user_id": "user_uuid",
  "messages": [
    {
      "id": "message_uuid",
      "sender_id": "user_uuid",
      "content": "Hello, how are you?",
      "message_type": "text",
      "message_status": "seen",
      "timestamp": 1640995200,
      "conversation_id": "conversation_uuid"
    }
  ],
  "limit": 50,
  "offset": 0,
  "total": 1
}
```

### 3. Get Group Chat History

Retrieve chat history for a specific group.

**Endpoint:** `GET /chat/group/:group_id/history`

**Parameters:**

- `group_id` (path): The UUID of the group
- `limit` (query, optional): Number of messages to return (default: 50, max: 100)
- `offset` (query, optional): Number of messages to skip (default: 0)

**Response:**

```json
{
  "conversation_id": "uuid",
  "conversation_type": "group",
  "group_id": "group_uuid",
  "messages": [
    {
      "id": "message_uuid",
      "sender_id": "user_uuid",
      "content": "Hello everyone!",
      "message_type": "text",
      "message_status": "sent",
      "timestamp": 1640995200,
      "conversation_id": "conversation_uuid",
      "group_id": "group_uuid"
    }
  ],
  "limit": 50,
  "offset": 0,
  "total": 1
}
```

## Error Responses

### Unauthorized (401)

```json
{
  "error": "Invalid token"
}
```

### Bad Request (400)

```json
{
  "error": "Invalid conversation ID"
}
```

### Internal Server Error (500)

```json
{
  "error": "Failed to get chat history"
}
```

## Usage Examples

### Get last 20 messages from a conversation

```bash
curl -H "Authorization: Bearer <token>" \
     "http://localhost:8080/chat/history/123e4567-e89b-12d3-a456-426614174000?limit=20"
```

### Get chat history with a specific user

```bash
curl -H "Authorization: Bearer <token>" \
     "http://localhost:8080/chat/history/with/456e7890-e89b-12d3-a456-426614174000"
```

### Get group chat history with pagination

```bash
curl -H "Authorization: Bearer <token>" \
     "http://localhost:8080/chat/group/789e0123-e89b-12d3-a456-426614174000/history?limit=25&offset=50"
```

## Notes

- Messages are returned in chronological order (oldest first)
- The `content` field contains the actual message text (no encryption)
- `message_status` can be "sent", "delivered", or "seen"
- `message_type` can be "text", "image", "file", etc.
- Pagination is supported with `limit` and `offset` parameters
- Maximum limit is 100 messages per request
