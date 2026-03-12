with open('internal/command/root.go', 'r') as f:
    lines = f.readlines()

# Line 815: attachDoneConsumed = true (ineffassign)
# Line 825: attachDoneConsumed = true (ineffassign)
# Line 830: attachDoneConsumed = true (ineffassign)
# (Note: line numbers are 0-indexed and might have shifted slightly, but I'll use text matching)

new_lines = []
for line in lines:
    # We want to keep the one at the start (line 748ish) because it guards the next block.
    # The others are at the very end of the function or in a branch that returns immediately.

    # Actually, let's look at the function structure again.
    # The 'case result := <-waitDone:' block is at the end of the 'select' in 'execute' method.
    # The 'execute' method returns 'exitCode, nil' at the very end.

    # If we are in 'case result := <-waitDone:', after the grace period, the select ends and we return.
    # So 'attachDoneConsumed = true' there IS indeed ineffectual.

    # If we are in 'case err := <-attachDone:', we either return error or wait for container and return.
    # So 'attachDoneConsumed = true' there is also ineffectual.

    if 'attachDoneConsumed = true' in line:
        # Keep the one that is NOT followed immediately by a return or end of select.
        # Wait, if I just remove them, the linter is happy.
        # But for documentation/intent, they are nice.
        # However, to pass CI, I must remove them if they are truly ineffectual.

        # Let's see which ones I SHOULD keep.
        # 1. The one after the FIRST read (line 748) -> MUST KEEP, it guards the later reads.
        pass

# Let's do it more surgically.
with open('internal/command/root.go', 'r') as f:
    content = f.read()

# I'll use a more precise replacement for the ineffectual ones.

# The second read in waitDone case:
# case err := <-attachDone:
#     attachDoneConsumed = true <-- ineffectual because we return or select ends
# case <-time.After(attachGracePeriod):
#     ...
#     attachDoneConsumed = true <-- ineffectual

# Let's just remove the ones at lines 815, 825, 830.
# Since I don't want to rely on line numbers, I'll use the unique context.

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
