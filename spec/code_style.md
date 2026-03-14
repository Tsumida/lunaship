# Golang code style

## Using framework

We prefer using `github.com/tsumida/lunaship`. It offers:
- Protobuf based API definition and code generation. Using `buf` to generate code can avoid a lot of boilerplate and make the code more consistent.
- Grpc based RPC framework. It has good performance and is widely used in the industry.
- Logging, Metric & Tracing infrastructure for MySQL, Redis invocations. 
- Kafka consumer config and handler. 
- Other basic utils.

## Simplicity and readability

- Function has less than 100 lines of code. If a function is too long, we can split it into smaller functions.
- Keep "SOLID" principle in mind. Each function should have a single responsibility and do it well.
- Function should have less then 6 parameters. Else we use a struct to wrap them.

## Concurrency safety

- Add hint to function parameter if they may be changed, like `ptr *int, //may update`. 

## Don't reinvent the wheel
Sometimes we need ad-hoc transformation of data, we can use `lo` to make the code more concise and readable.

For example: 
```go
arr := []int{1, 2, 3, 4, 5}
squared := lo.Map(arr, func(x int) int {
    return x * x
})
```
If you have to reinvent the wheel, please ask the user for advises and decide together.

## Error Handling
Each function that interacts with external systems (e.g., database, network), or maybe fail due to invalid input, should return an error.

Function may have multiple exit points, make them easy to distinguish. For example,
```go
func Deal(ctx context.Context, input string) (string, error){
    if func_a(input){
        log.Error("func_a failed", zap.String("input", input))
        return "", fmt.Errorf("func_a failed")
    }
    if func_b(input){
        log.Error("func_b failed", zap.String("input", input))
        return "", fmt.Errorf("func_b failed")
    }
    return "success", nil
}
```
Or use different errors:
```go
// or 
func Deal(ctx context.Context, input string) (string, error){
    if func_a(input){
        return "", ERR_FUNC_A_FAILED
    }
    if func_b(input){
        return "", ERR_FUNC_B_FAILED
    }
    return "success", nil
}
```


## Test
Each Test function should have  simple comments about description, procedure and the expectation of the test, so User can understand the test without reading the code. 
For example: 
```go
func TestF(t *testing.T) {
    // Description:  f(a) + f(b) == f(a + b)
    // Procedure: We generate 1000 random pairs (a, b) and check the invariant above.
    // Expectation: The invariant should hold for all pairs.
    // ...
}
```

Prefer `assert` over if-else block. 

# Change logs
- Follow the best practices.
- Make attemption clear and concise. 
- Use summary and description to make the procedure reasonable and easy to understand. 
- Keep business logic and non-business logic separate. For example, we can put all initialization, environment variable reading in a signle `Prepare()` function. 