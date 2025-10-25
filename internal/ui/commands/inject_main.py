# Auto-injected by Cerebrium CLI
if __name__ == "__main__":
    import sys
    import json
    import os
    import inspect
    import traceback
    
    input_data = os.environ.get('CEREBRIUM_INPUT_DATA')
    data = None
    if input_data:
        try:
            data = json.loads(input_data)
        except json.JSONDecodeError as e:
            print(f"Error parsing input data: {e}", file=sys.stderr)
            sys.exit(1)
    
    main_func = None
    current_module = sys.modules.get('__main__')
    
    if current_module:
        # Look for common entry point function names
        entry_point_names = ['main', 'run', 'handler', 'process', 'execute', 'predict']
        for name in entry_point_names:
            if hasattr(current_module, name):
                obj = getattr(current_module, name)
                if callable(obj) and not name.startswith('_'):
                    main_func = obj
                    print(f"Found entry point: {name}", file=sys.stderr)
                    break
    
    if not main_func:
        print("Info: No entry point function (main, run, handler, process, execute, predict) found. Executed script top-level code.", file=sys.stderr)
        sys.exit(0)
    
    try:
        # Inspect function signature to handle parameters correctly
        sig = inspect.signature(main_func)
        params = list(sig.parameters.keys())
        
        if data is not None:
            if len(params) == 0:
                print("Warning: Entry point function takes no arguments, ignoring input data", file=sys.stderr)
                result = main_func()
            elif len(params) == 1 and params[0] not in ['self', 'cls']:
                result = main_func(data)
            elif isinstance(data, dict):
                # Filter kwargs to only include parameters the function accepts
                filtered_kwargs = {k: v for k, v in data.items() if k in params}
                if len(filtered_kwargs) < len(data):
                    ignored_keys = set(data.keys()) - set(filtered_kwargs.keys())
                    print(f"Warning: Ignoring unknown parameters: {ignored_keys}", file=sys.stderr)
                result = main_func(**filtered_kwargs)
            else:
                result = main_func(data)
        else:
            if len(params) > 0 and not all(p.default != inspect.Parameter.empty for p in sig.parameters.values()):
                print(f"Warning: Function expects parameters but none provided", file=sys.stderr)
            result = main_func()
        
        if result is not None:
            if isinstance(result, str):
                print(result)
            else:
                try:
                    print(json.dumps(result, default=str))
                except (TypeError, ValueError) as e:
                    # Fallback for non-JSON serializable objects
                    print(str(result))
                    
    except TypeError as e:
        if "missing" in str(e) and "required positional argument" in str(e):
            print(f"Error: Function signature mismatch. {e}", file=sys.stderr)
            print(f"Function parameters: {params}", file=sys.stderr)
            print(f"Provided data: {data}", file=sys.stderr)
        else:
            print(f"Error executing function: {e}", file=sys.stderr)
            traceback.print_exc(file=sys.stderr)
        sys.exit(1)
    except Exception as e:
        print(f"Error executing function: {e}", file=sys.stderr)
        traceback.print_exc(file=sys.stderr)
        sys.exit(1)
