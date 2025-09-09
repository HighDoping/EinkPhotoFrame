## Converted from https://gist.github.com/me-no-dev/f137a950ce6dedb641d427d8db6355d2 filetoarray.c

import sys
import os

def main():
    if len(sys.argv) != 2:
        print("Filename not supplied")
        return 1

    fname = sys.argv[1]
    pname = fname.replace(".", "_")

    try:
        with open(fname, "rb") as fp:
            buffer = fp.read()
    except FileNotFoundError:
        print(f"File not found: {fname}")
        return 1

    flen = len(buffer)
    header_name = f"{pname}.h"

    with open(header_name, "w") as out:
        out.write(f"// File: {fname}, Size: {flen}\n")
        out.write(f"#define {pname}_len {flen}\n")
        out.write(f"const uint8_t {pname}[] PROGMEM = {{\n")

        for i, b in enumerate(buffer):
            end = "," if i < flen - 1 else ""
            out.write(f" 0x{b:02X}{end}")
            if i % 16 == 15:
                out.write("\n")

        out.write("\n};\n")

    print(f"Header file written: {header_name}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
