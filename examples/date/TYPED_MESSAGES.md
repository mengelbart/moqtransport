# Typed Messages Update

This example has been updated to use the new typed messages pattern in moqtransport.

## Key Changes

1. **Typed Handler Implementation**
   - Created `dateTypedHandler` that implements `TypedHandler` interface
   - Uses `HandleTyped` method with type-safe message handling

2. **Type-Safe Message Processing**
   - Uses type assertions to handle specific message types:
     - `*moqtransport.AnnounceMessage` 
     - `*moqtransport.SubscribeMessage`
   - Direct access to all message fields without string comparisons

3. **Enhanced Response Writers**
   - Uses `AnnounceResponseWriter` for detailed announce responses
   - Uses `SubscribeResponseWriter` for detailed subscribe responses
   - Supports specific error codes like `SubscribeErrorTrackDoesNotExist`

4. **Improved Logging**
   - Logs all subscription parameters:
     - Track Alias
     - Subscriber Priority
     - Group Order preference
     - Filter Type

5. **Backward Compatibility**
   - Falls back to basic `Accept()`/`Reject()` if enhanced writers not available
   - Uses `HandlerAdapter` to work with existing infrastructure

## Benefits

- **Type Safety**: Compile-time checking of message fields
- **Better Error Handling**: Specific error codes for different scenarios
- **More Control**: Can set response parameters like group order
- **Cleaner Code**: No string-based method checking

## Example Output

When receiving a subscription, you'll now see:
```
Subscribe request for example-namespace/date-track
  Track Alias: 1
  Priority: 128
  Group Order: 0
  Filter Type: 1
```

This provides much more visibility into the client's subscription preferences.