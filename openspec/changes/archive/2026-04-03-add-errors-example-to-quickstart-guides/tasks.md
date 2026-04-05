## 1. Update checkout.idl

- [x] 1.1 Add `errors {}` block with CartNotFound, CartEmpty, PaymentFailed, OutOfStock, InvalidAddress error declarations
- [x] 1.2 Add `raises()` clause to `createOrder` method on OrderService interface
- [x] 1.3 Remove error code comments (lines 81-86)

## 2. Update language quickstarts

- [x] 2.1 Add "Defining Service Errors" section to Python quickstart showing errors keyword syntax
- [x] 2.2 Add "Defining Service Errors" section to Go quickstart showing errors keyword syntax
- [x] 2.3 Add "Defining Service Errors" section to TypeScript quickstart showing errors keyword syntax
- [x] 2.4 Add "Defining Service Errors" section to Java quickstart showing errors keyword syntax
- [x] 2.5 Add "Defining Service Errors" section to C# quickstart showing errors keyword syntax

## 3. Verify

- [x] 3.1 Confirm all five language quickstarts show the errors keyword and raises() syntax
- [x] 3.2 Confirm links to errors documentation are present
