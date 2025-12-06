import argparse
import json
import sys

if __name__ == "__main__":
    script_name = sys.argv[0]

    if len(sys.argv) < 2:
        print(f'Usage: python {script_name} <function_name> [--data \'{{"key": "value"}}\']')
        sys.exit(1)

    func_name = sys.argv[1]

    # If no function name is provided run the script without executing any function
    if func_name == "":
        sys.exit(0)

    func = globals().get(func_name)

    if not callable(func):
        print(f"Function '{func_name}' not found.")
        sys.exit(1)

    parser = argparse.ArgumentParser(prog=f"{script_name} {func_name}")
    parser.add_argument(
        "--data", type=str, help="Optional JSON object to pass as keyword arguments"
    )

    args = parser.parse_args(sys.argv[2:])

    if args.data:
        try:
            kwargs = json.loads(args.data)
            if not isinstance(kwargs, dict):
                raise ValueError("Data must be a JSON object")
        except Exception as e:
            print(f"Invalid JSON in --data: {e}")
            sys.exit(1)
    else:
        kwargs = {}

    func(**kwargs)
