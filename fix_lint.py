with open('internal/command/root.go', 'r') as f:
    content = f.read()

content = content.replace(
    'case err := <-attachDone:\n\t\t\t\tattachDoneConsumed = true\n\t\t\t\tif err != nil',
    'case err := <-attachDone:\n\t\t\t\tif err != nil'
)

content = content.replace(
    '<-attachDone\n\t\t\t\tattachDoneConsumed = true',
    '<-attachDone'
)

content = content.replace(
    'case err := <-attachDone:\n\t\tattachDoneConsumed = true\n\t\tif err != nil',
    'case err := <-attachDone:\n\t\tif err != nil'
)

with open('internal/command/root.go', 'w') as f:
    f.write(content)
