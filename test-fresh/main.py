def run(prompt: str):
    print(f"Running on Cerebrium: {prompt}")

    return {"my_result": prompt}

# To run your app, run:
# cerebrium run main.py::run --prompt "Hello World"
#
# To deploy your app, run:
# cerebrium deploy
