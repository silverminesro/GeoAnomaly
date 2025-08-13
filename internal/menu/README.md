# Menu System Documentation

## Overview

The menu system provides a comprehensive in-game economy with two currencies:
- **Credits**: Earned through gameplay, selling items, and rewards
- **Essence**: Premium currency purchased with real money

## Features

### 🪙 Currency System
- **Credits**: Default 5000 credits for new users
- **Essence**: Premium currency for real money purchases
- JWT-protected transactions
- Complete transaction history

### 🛒 Market System
- Purchase items with credits or essence
- Item categories: artifacts, gear, consumables, cosmetics
- Rarity system: common, rare, epic, legendary
- Tier and level requirements
- Stock management for limited items

### 💎 Essence Packages
- Real money purchases (USD, EUR, GBP)
- Bonus essence for larger packages
- Payment status tracking
- Secure transaction handling

### 📦 Inventory Selling
- Sell artifacts and gear for credits
- Price calculation based on rarity and type
- Automatic inventory cleanup

## API Endpoints

### Currency Management
```
GET /api/v1/menu/currency/all          # Get all user currencies
GET /api/v1/menu/currency/{type}       # Get specific currency (credits/essence)
```

### Market System
```
GET /api/v1/menu/market/items          # Get available market items
POST /api/v1/menu/market/purchase      # Purchase market item
```

### Essence Packages
```
GET /api/v1/menu/essence/packages      # Get available essence packages
POST /api/v1/menu/essence/purchase     # Purchase essence package
```

### Inventory Selling
```
POST /api/v1/menu/inventory/{id}/sell  # Sell inventory item for credits
```

### Transaction History
```
GET /api/v1/menu/transactions          # Get transaction history
GET /api/v1/menu/purchases             # Get purchase history
GET /api/v1/menu/essence/purchases     # Get essence purchase history
```

### Admin Endpoints (Admin Only)
```
POST /api/v1/admin/menu/market/items           # Create market item
PUT /api/v1/admin/menu/market/items/{id}      # Update market item
DELETE /api/v1/admin/menu/market/items/{id}   # Delete market item

POST /api/v1/admin/menu/essence/packages      # Create essence package
PUT /api/v1/admin/menu/essence/packages/{id}  # Update essence package
DELETE /api/v1/admin/menu/essence/packages/{id} # Delete essence package
```

## Database Models

### Currency
- User ID and currency type
- Current amount
- Last updated timestamp

### Transaction
- User ID and transaction type
- Currency type and amount
- Balance before and after
- Item reference (optional)
- Description

### MarketItem
- Name, description, type, category
- Rarity and level
- Credits and essence prices
- Availability settings
- Tier and level requirements
- Media URLs

### UserPurchase
- Links user to purchased market item
- Quantity and payment details
- Transaction reference

### EssencePackage
- Package details and pricing
- Real money prices in cents
- Bonus essence amounts
- Availability settings

### UserEssencePurchase
- Links user to essence package
- Payment method and currency
- Payment status and amounts
- External payment reference

## Security Features

- **JWT Authentication**: All endpoints require valid JWT token
- **User Validation**: Users can only access their own data
- **Admin Protection**: Admin endpoints require admin privileges
- **Transaction Logging**: All currency changes are logged
- **Payment Validation**: Real money amounts are validated

## Price Calculation

### Selling Items
Base prices by item type:
- Artifacts: 100 credits
- Gear: 150 credits

Rarity multipliers:
- Common: 1.0x
- Rare: 2.0x
- Epic: 5.0x
- Legendary: 10.0x

### Market Items
- Set by admins via admin endpoints
- Can be priced in credits, essence, or both
- Dynamic pricing based on rarity and category

## Usage Examples

### Get User Currencies
```bash
curl -H "Authorization: Bearer <jwt_token>" \
     http://localhost:8080/api/v1/menu/currency/all
```

### Purchase Market Item
```bash
curl -X POST -H "Authorization: Bearer <jwt_token>" \
     -H "Content-Type: application/json" \
     -d '{
       "item_id": "uuid-here",
       "quantity": 1,
       "currency_type": "credits"
     }' \
     http://localhost:8080/api/v1/menu/market/purchase
```

### Sell Inventory Item
```bash
curl -X POST -H "Authorization: Bearer <jwt_token>" \
     http://localhost:8080/api/v1/menu/inventory/uuid-here/sell
```

### Purchase Essence Package
```bash
curl -X POST -H "Authorization: Bearer <jwt_token>" \
     -H "Content-Type: application/json" \
     -d '{
       "package_id": "uuid-here",
       "payment_method": "stripe",
       "payment_currency": "USD",
       "payment_amount": 999
     }' \
     http://localhost:8080/api/v1/menu/essence/purchase
```

## Database Indexes

The system creates optimized indexes for:
- Currency queries by user and type
- Transaction history by user
- Market items by category and rarity
- User purchases by user
- Essence packages by active status
- User essence purchases by user

## Error Handling

Common error responses:
- `400 Bad Request`: Invalid parameters
- `401 Unauthorized`: Missing or invalid JWT
- `403 Forbidden`: Insufficient permissions
- `404 Not Found`: Item not found
- `409 Conflict`: Insufficient funds
- `500 Internal Server Error`: Database or server error

## Future Enhancements

- [ ] Discount system for bulk purchases
- [ ] Limited-time offers and events
- [ ] Gift system between players
- [ ] Auction house functionality
- [ ] Currency conversion rates
- [ ] Advanced payment gateways
- [ ] Analytics and reporting
- [ ] Mobile payment integration 