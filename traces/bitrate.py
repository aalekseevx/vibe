import sys


def calculate_bitrate(filename):
    total_bytes = 0
    max_timestamp = 0

    with open(filename, "r") as file:
        # Skip the header line
        next(file)

        for line in file:
            # Skip lines starting with '%'
            if line.startswith("%"):
                continue

            # Split the line and extract size and timestamp
            parts = line.strip().split()
            if len(parts) >= 5:  # Ensure line has enough components
                size = int(parts[4])
                timestamp = float(parts[3])

                total_bytes += size
                max_timestamp = max(max_timestamp, timestamp)

    # Calculate average bitrate in bits per second
    if max_timestamp > 0:
        bitrate = (total_bytes * 8) / max_timestamp  # Convert bytes to bits
        return total_bytes, max_timestamp, bitrate
    else:
        return 0, 0, 0


def main():
    if len(sys.argv) != 2:
        print("Usage: python script.py <trace_file>")
        sys.exit(1)

    filename = sys.argv[1]
    try:
        total_bytes, duration, bitrate = calculate_bitrate(filename)
        print(f"Total bytes: {total_bytes:,} bytes")
        print(f"Video duration: {duration:.2f} seconds")
        print(f"Average bitrate: {bitrate/1000:.2f} kbps")
    except FileNotFoundError:
        print(f"Error: File '{filename}' not found")
        sys.exit(1)
    except Exception as e:
        print(f"Error processing file: {e}")
        sys.exit(1)


if __name__ == "__main__":
    main()
