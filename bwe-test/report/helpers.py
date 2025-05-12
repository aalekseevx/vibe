
import pandas as pd
import yaml
import numpy as np
from matplotlib import pyplot as plt
from matplotlib.collections import PolyCollection
import matplotlib.dates as mdates
from pathlib import Path
from datetime import timedelta

def load_config(config_path):
    """Load the configuration file and return test cases as a dictionary."""
    with open(config_path, 'r') as f:
        return yaml.safe_load(f)
    
def config_to_test_cases(config):
    test_cases = {}
    for test_case in config.get('test_cases', []):
        if 'name' in test_case:
            test_cases[test_case['name']] = test_case
    
    return test_cases

def extract_path_characteristics(test_case, config):
    """Extract path characteristics from the test case configuration.
    
    If a path_characteristic_preset is specified in the test case, it will be used to look up
    the path characteristics from the config's path_characteristic_presets.
    
    Args:
        test_case: The test case configuration.
        config: The full configuration containing path_characteristic_presets.
    """
    preset_name = test_case.get('path_characteristic_preset')
    path_char = config['path_characteristic_presets'].get(preset_name, {})    
    phases = path_char.get('phases', [])
    
    if not phases:
        return None
    
    # Convert phases to a time series
    time_points = []
    capacity_points = []
    
    current_time = 0
    
    for phase in phases:
        # Extract duration in seconds
        duration_str = phase.get('duration', '0s')
        if duration_str.endswith('s'):
            duration = int(duration_str[:-1])
        else:
            duration = int(duration_str)
        
        # Extract capacity in bits per second
        capacity = phase.get('capacity', 0)
        
        # Add start point of this phase
        time_points.append(current_time)
        capacity_points.append(capacity / 1000)  # Convert to kbps
        
        # Add end point of this phase
        current_time += duration
        time_points.append(current_time)
        capacity_points.append(capacity / 1000)  # Convert to kbps
    
    return {
        'time': time_points,
        'capacity_kbps': capacity_points
    }

def get_quality_name_from_csrc(config, test_cases, experiment, csrc):
    """Get the quality name for a given CSRC from the test case configuration."""
    if experiment not in test_cases:
        return f"Unknown (CSRC: {csrc})"
    
    test_case = test_cases[experiment]
    sender_config = test_case.get('sender', {})
    
    if sender_config.get('mode') != 'simulcast':
        return f"Default (CSRC: {csrc})"
    
    simulcast_presets = sender_config.get('simulcast_presets', [])
    simulcast_configs = config.get('simulcast_configs_presets', {})
    
    for preset_name in simulcast_presets:
        if preset_name in simulcast_configs:
            preset = simulcast_configs[preset_name]
            qualities = preset.get('qualities', [])
            
            for quality in qualities:
                if quality.get('csrc') == csrc:
                    return quality.get('name', f"Unknown (CSRC: {csrc})")
    
    return f"Unknown (CSRC: {csrc})"

def get_quality_bitrates(config, test_cases, experiment):
    """Get all qualities and their bitrates for an experiment.
    
    Args:
        config: The full configuration.
        test_cases: Dictionary of test cases.
        experiment: The experiment name.
        
    Returns:
        A list of dictionaries with 'name' and 'bitrate' keys, sorted by bitrate.
    """
    if experiment not in test_cases:
        return []
    
    test_case = test_cases[experiment]
    sender_config = test_case.get('sender', {})
    
    if sender_config.get('mode') != 'simulcast':
        return []
    
    simulcast_presets = sender_config.get('simulcast_presets', [])
    simulcast_configs = config.get('simulcast_configs_presets', {})
    
    # Find all qualities and their bitrates
    all_qualities = []
    for preset_name in simulcast_presets:
        if preset_name in simulcast_configs:
            preset = simulcast_configs[preset_name]
            qualities = preset.get('qualities', [])
            
            for quality in qualities:
                name = quality.get('name')
                bitrate = quality.get('bitrate', 0)
                if name and name not in [q['name'] for q in all_qualities]:
                    all_qualities.append({'name': name, 'bitrate': bitrate})
    
    # Sort qualities by bitrate
    return sorted(all_qualities, key=lambda q: q['bitrate'])


def load_experiments_data(experiments, base_path):
    """Load experiment data from log files, hardcoded to flow 0 only."""
    experiment_data = {}

    for experiment in experiments:
        # Construct full path
        exp_path = Path(base_path) / experiment
        flow_id = 0  # Hardcoded to flow 0
        logs = {}

        # Find all log files for flow 0
        for log_file in exp_path.glob(f"{flow_id}_*.log"):
            parts = log_file.stem.split("_", 1)
            if len(parts) != 2:
                continue  # skip malformed names
            _, log_type = parts
            logs[log_type] = log_file

        # Parse logs for flow 0
        flow_data = {}

        if "cc" in logs:
            cc_log = pd.read_csv(logs["cc"], header=None, names=["time", "target_bitrate"])
            cc_log["time"] = pd.to_datetime(cc_log["time"], unit="ms")
            flow_data["cc_log"] = cc_log

        for side in ["sender", "receiver"]:
            for kind in ["rtp", "rtcp"]:
                key = f"{side}_{kind}"
                if key in logs:
                    if kind == "rtp":
                        def string_to_bool(value):
                            if isinstance(value, str):
                                return value.lower() == 'true'
                            return bool(value)


                        df = pd.read_csv(logs[key], header=None, names=[
                            "time", "payload_type", "ssrc", "seq", "timestamp",
                            "marker", "size", "twcc", "unwrapped_seq", "csrc", "isRTX", "isFEC"
                        ], converters={'isRTX': string_to_bool, 'isFEC': string_to_bool})

                        df["time"] = pd.to_datetime(df["time"], unit="ms")
                    else:
                        df = pd.read_csv(logs[key], header=None, names=["time", "size"])
                        df["time"] = pd.to_datetime(df["time"], unit="ms")
                    flow_data[key] = df

        experiment_data[experiment] = flow_data

    return experiment_data

def compute_bitrates(experiment_data, window_ms=500):
    """Compute bitrates for all experiments."""
    for exp_name, data in experiment_data.items():
        if "sender_rtp" in data:
            df = data["sender_rtp"].copy()
            df["time_bin"] = df["time"].dt.floor(f"{window_ms}ms")
            
            # Compute overall bitrate
            bitrate_df = df.groupby("time_bin")["size"].sum().reset_index()
            bitrate_df["bitrate_kbps"] = (bitrate_df["size"] * 8) / (window_ms / 1000) / 1000
            data["bitrate"] = bitrate_df
            
            # Compute bitrate per SSRC
            ssrc_bitrates = {}
            for ssrc, group in df.groupby("ssrc"):
                ssrc_df = group.groupby("time_bin")["size"].sum().reset_index()
                ssrc_df["bitrate_kbps"] = (ssrc_df["size"] * 8) / (window_ms / 1000) / 1000
                ssrc_bitrates[ssrc] = ssrc_df
            data["ssrc_bitrates"] = ssrc_bitrates

def compute_lost_packets(experiment_data):
    """Compute cumulative lost packets for all experiments, separated by SSRC."""
    for exp_name, data in experiment_data.items():
        if "receiver_rtp" in data:
            df = data["receiver_rtp"]
            
            # Calculate lost packets per SSRC
            ssrc_lost_packets = {}
            
            for ssrc, group in df.groupby("ssrc"):
                # Sort by unwrapped sequence number to ensure correct order
                group = group.sort_values("unwrapped_seq")
                
                # Calculate min unwrapped sequence number for this SSRC
                min_seq = group["unwrapped_seq"].min()
                
                # Calculate expected packets at each point
                group["expected_packets"] = group["unwrapped_seq"] - min_seq + 1
                
                # Calculate received packets (cumulative count)
                group["received_packets"] = np.arange(1, len(group) + 1)
                
                # Calculate lost packets
                group["lost_packets"] = group["expected_packets"] - group["received_packets"]
                
                # Store the result
                ssrc_lost_packets[ssrc] = group[["time", "lost_packets"]]
            
            data["ssrc_lost_packets"] = ssrc_lost_packets

def compute_interarrival_jitter(experiment_data):
    """Compute interarrival jitter according to RFC 3550, separated by SSRC."""
    for exp_name, data in experiment_data.items():
        if "receiver_rtp" in data:
            df = data["receiver_rtp"]
            
            # Calculate jitter per SSRC
            ssrc_jitter = {}
            
            for ssrc, group in df.groupby("ssrc"):
                # Sort by time to ensure correct order
                group = group.sort_values("time")
                
                # Convert arrival time to milliseconds since first packet
                first_arrival = group["time"].min()
                group["arrival_ms"] = (group["time"] - first_arrival).dt.total_seconds() * 1000
                
                # Initialize jitter array
                jitter = np.zeros(len(group))
                
                # Calculate jitter for each packet (starting from the second one)
                for i in range(1, len(group)):
                    # Calculate D: difference in relative transit times
                    # D(i-1,i) = (Ri - Ri-1) - (Si - Si-1) = (Ri - Si) - (Ri-1 - Si-1)
                    arrival_diff = group.iloc[i]["arrival_ms"] - group.iloc[i-1]["arrival_ms"]
                    timestamp_diff = group.iloc[i]["timestamp"] - group.iloc[i-1]["timestamp"]
                    
                    # Convert timestamp diff to same units as arrival (ms)
                    # Assuming RTP timestamp is in the same timebase
                    # This is a simplification - in reality, we would need to know the clock rate
                    timestamp_diff_ms = timestamp_diff / 90  # Assuming 90kHz clock rate for video
                    
                    D = arrival_diff - timestamp_diff_ms
                    
                    # Update jitter using the formula J(i) = J(i-1) + (|D(i-1,i)| - J(i-1))/16
                    jitter[i] = jitter[i-1] + (abs(D) - jitter[i-1]) / 16
                
                # Create a DataFrame with time and jitter
                jitter_df = pd.DataFrame({
                    "time": group["time"],
                    "jitter": jitter
                })
                
                # Store the result
                ssrc_jitter[ssrc] = jitter_df
            
            data["ssrc_jitter"] = ssrc_jitter

def compute_one_way_delay(experiment_data):
    """Compute one-way delay of RTP packets as the difference between timestamps in sender_rtp and receiver_rtp.
    Uses twccNr for matching packets between sender and receiver instead of unwrappedSeqNr."""
    for exp_name, data in experiment_data.items():
        if "sender_rtp" in data and "receiver_rtp" in data:
            sender_df = data["sender_rtp"]
            receiver_df = data["receiver_rtp"]
            
            # Merge sender and receiver data based on twccNr
            merged_df = pd.merge(
                sender_df[["time", "ssrc", "twcc", "timestamp"]],
                receiver_df[["time", "ssrc", "twcc", "timestamp"]],
                on="twcc",
                suffixes=("_sender", "_receiver")
            )
            
            if not merged_df.empty:
                # Calculate one-way delay in milliseconds
                merged_df["one_way_delay_ms"] = (
                    (merged_df["time_receiver"] - merged_df["time_sender"]).dt.total_seconds() * 1000
                )
                
                # Store the result as a single DataFrame
                data["one_way_delay"] = merged_df[["time_receiver", "ssrc_sender", "one_way_delay_ms"]]

def create_quality_timeline(experiment_data, config, test_cases):
    """Create a timeline of quality changes for each stream (SSRC).
    
    This function tracks all CSRCs for each SSRC and creates a new timeline entry
    whenever the CSRC (and thus quality) changes for an SSRC.
    
    RTX and FEC packets are excluded from the timeline.
    """
    for exp_name, data in experiment_data.items():
        if "sender_rtp" in data:
            df = data["sender_rtp"]

            print(df['isRTX'].describe())
            df = df[~df["isRTX"] & ~df["isFEC"]]

            # Create a timeline DataFrame
            timeline_data = []
            
            # Process each SSRC separately
            for ssrc, ssrc_df in df.groupby("ssrc"):
                # Sort by time to ensure chronological order
                ssrc_df = ssrc_df.sort_values("time")
                
                # Track quality changes by monitoring CSRC changes
                current_csrc = None
                segment_start = None
                
                for _, row in ssrc_df.iterrows():
                    time = row["time"]
                    csrc = row["csrc"]
                    
                    # If this is the first packet or CSRC has changed
                    if current_csrc is None or csrc != current_csrc:
                        # If we were tracking a previous segment, close it
                        if current_csrc is not None and segment_start is not None:
                            quality_name = get_quality_name_from_csrc(config, test_cases, exp_name, current_csrc)
                            
                            timeline_data.append({
                                "SSRC": str(ssrc),
                                "Quality": quality_name,
                                "Start": segment_start,
                                "Finish": time,  # End at the current time
                                "CSRC": current_csrc
                            })
                        
                        # Start a new segment
                        current_csrc = csrc
                        segment_start = time
                
                # Close the final segment if there is one
                if current_csrc is not None and segment_start is not None:
                    quality_name = get_quality_name_from_csrc(config, test_cases, exp_name, current_csrc)
                    
                    timeline_data.append({
                        "SSRC": str(ssrc),
                        "Quality": quality_name,
                        "Start": segment_start,
                        "Finish": ssrc_df["time"].max(),  # End at the last packet time
                        "CSRC": current_csrc
                    })
            
            # Create a DataFrame for the timeline
            timeline_df = pd.DataFrame(timeline_data)
            data["quality_timeline"] = timeline_df

def plot_experiment_results(config, experiment_data, path_characteristics_map, test_cases):
    """Plot bitrates, path characteristics, lost packets, jitter, one-way delay, and quality timeline for all experiments."""
    for exp_name, data in experiment_data.items():
        # Check if this experiment has simulcast mode
        is_simulcast = False
        if exp_name in test_cases:
            test_case = test_cases[exp_name]
            sender_config = test_case.get('sender', {})
            is_simulcast = sender_config.get('mode') == 'simulcast'
        
        # Determine number of subplots based on whether we're showing quality timeline and one-way delay
        has_one_way_delay = "one_way_delay" in data and not data["one_way_delay"].empty
        
        if is_simulcast and has_one_way_delay:
            # Create a figure with five subplots (including quality timeline and one-way delay)
            fig, (ax_timeline, ax1, ax2, ax3, ax4) = plt.subplots(5, 1, figsize=(12, 24), 
                                                                gridspec_kw={'height_ratios': [1, 2, 2, 2, 2]})
        elif is_simulcast:
            # Create a figure with four subplots (including quality timeline)
            fig, (ax_timeline, ax1, ax2, ax3) = plt.subplots(4, 1, figsize=(12, 20), 
                                                           gridspec_kw={'height_ratios': [1, 2, 2, 2]})
        elif has_one_way_delay:
            # Create a figure with four subplots (including one-way delay)
            fig, (ax1, ax2, ax3, ax4) = plt.subplots(4, 1, figsize=(12, 20))
        else:
            # Create a figure with three subplots (no quality timeline, no one-way delay)
            fig, (ax1, ax2, ax3) = plt.subplots(3, 1, figsize=(12, 16))
        
        fig.suptitle(f"Experiment Results - {exp_name}", fontsize=16)
        
        # Create formatter for x-axis time display
        locator = mdates.AutoDateLocator()
        formatter = mdates.ConciseDateFormatter(locator)
        
        # Plot quality timeline using matplotlib only for simulcast mode
        if is_simulcast and "quality_timeline" in data:
            timeline_df = data["quality_timeline"]
            
            # Get unique SSRCs and qualities
            unique_ssrcs = timeline_df["SSRC"].unique()
            unique_qualities = timeline_df["Quality"].unique()
            
            # Create a mapping of SSRC to category numbers (Y-axis positions)
            ssrc_cats = {ssrc: i+1 for i, ssrc in enumerate(unique_ssrcs)}
            
            # Get all qualities sorted by bitrate
            sorted_qualities = get_quality_bitrates(config, test_cases, exp_name)
            
            # Create a color mapping for qualities based on bitrate level
            quality_colormapping = {}
            for quality in unique_qualities:
                # Find the position of this quality in the sorted list
                position = -1
                for i, q in enumerate(sorted_qualities):
                    if q['name'] == quality:
                        position = i
                        break
                
                # Assign color based on position
                if position == 0:  # Lowest bitrate
                    quality_colormapping[quality] = 'red'
                elif position == len(sorted_qualities) - 1:  # Highest bitrate
                    quality_colormapping[quality] = 'green'
                elif position > 0:  # Medium bitrate(s)
                    quality_colormapping[quality] = 'yellow'
                else:  # Unknown quality
                    quality_colormapping[quality] = 'blue'
            
            # Create vertices and colors for PolyCollection
            verts = []
            bar_colors = []
            
            # Create a list to store quality-color pairs for the legend
            legend_elements = []
            
            for _, row in timeline_df.iterrows():
                ssrc = row["SSRC"]
                quality = row["Quality"]
                start_time = row["Start"]
                end_time = row["Finish"]
                
                # Create a rectangle for this timeline entry
                v = [
                    (mdates.date2num(start_time), ssrc_cats[ssrc] - 0.4),
                    (mdates.date2num(start_time), ssrc_cats[ssrc] + 0.4),
                    (mdates.date2num(end_time), ssrc_cats[ssrc] + 0.4),
                    (mdates.date2num(end_time), ssrc_cats[ssrc] - 0.4),
                    (mdates.date2num(start_time), ssrc_cats[ssrc] - 0.4)
                ]
                verts.append(v)
                
                # Color is based on quality
                color = quality_colormapping[quality]
                bar_colors.append(color)
                
                # Add to legend if not already there
                from matplotlib.patches import Patch
                if quality not in [e.get_label() for e in legend_elements]:
                    legend_elements.append(Patch(facecolor=color, label=quality))
            
            # Create the PolyCollection and add it to the timeline axis
            bars = PolyCollection(verts, facecolors=bar_colors)
            ax_timeline.add_collection(bars)
            ax_timeline.autoscale()
            
            # Set up the timeline axis
            ax_timeline.set_title("Quality Timeline")
            ax_timeline.set_yticks(list(ssrc_cats.values()))
            ax_timeline.set_yticklabels(list(ssrc_cats.keys()))
            
            # Add a legend for quality colors
            ax_timeline.legend(handles=legend_elements, loc='upper right')
            
        # Apply formatter to timeline x-axis if simulcast mode
        if is_simulcast:
            ax_timeline.xaxis.set_major_locator(locator)
            ax_timeline.xaxis.set_major_formatter(formatter)
        
        # Setup first subplot for bitrate
        ax1.set_title("Bitrate Utilization")
        ax1.set_ylabel("Bitrate (kbps)")
        
        # Setup second subplot for lost packets
        ax2.set_title("Cumulative Lost Packets")
        ax2.set_ylabel("Lost Packets Count")
        
        # Setup third subplot for jitter
        ax3.set_title("Interarrival Jitter (RFC 3550)")
        ax3.set_ylabel("Jitter (ms)")
        
        # Setup fourth subplot for one-way delay if available
        if has_one_way_delay:
            ax4.set_title("One-Way Delay")
            ax4.set_xlabel("Time")
            ax4.set_ylabel("Delay (ms)")
        else:
            # If no one-way delay, set x-label on jitter subplot
            ax3.set_xlabel("Time")
        
        colors = plt.colormaps.get_cmap('tab10').colors

        # Plot path characteristics if available for this experiment
        path_characteristics = path_characteristics_map.get(exp_name)
        if path_characteristics:
            # Get the start time from the data
            start_time = None
            if "sender_rtp" in data:
                start_time = data["sender_rtp"]["time"].min()
            
            if start_time is not None:
                # Convert seconds to datetime
                time_points = [start_time + timedelta(seconds=t) for t in path_characteristics['time']]
                ax1.plot(time_points, path_characteristics['capacity_kbps'], 
                        label="Path Capacity", color='black', linestyle='-.', linewidth=2)

        # Plot RTP bitrate on first subplot
        if "bitrate" in data:
            df = data["bitrate"]
            ax1.plot(df["time_bin"], df["bitrate_kbps"], label="RTP Bitrate",
                    color=colors[0], linestyle='-')

        # Plot CC target on first subplot
        if "cc_log" in data:
            cc = data["cc_log"]
            ax1.plot(cc["time"], cc["target_bitrate"] / 1000, label="Target Bitrate",
                    color=colors[1], linestyle='--')
        
        # Plot SSRC-specific data
        if "ssrc_lost_packets" in data:
            for i, (ssrc, lost_df) in enumerate(data["ssrc_lost_packets"].items()):
                color = colors[(i + 2) % len(colors)]
                
                # Plot lost packets
                ax2.plot(lost_df["time"], lost_df["lost_packets"], 
                        label=f"SSRC {ssrc} Lost Packets",
                        color=color, linestyle='-')
        
        if "ssrc_jitter" in data:
            for i, (ssrc, jitter_df) in enumerate(data["ssrc_jitter"].items()):
                color = colors[(i + 2) % len(colors)]
                
                # Plot jitter
                ax3.plot(jitter_df["time"], jitter_df["jitter"], 
                        label=f"SSRC {ssrc} Jitter",
                        color=color, linestyle='-')

        # Add legends
        ax1.legend()
        ax2.legend()
        ax3.legend()
        
        # Format x-axis for all subplots to use the same time scale
        if has_one_way_delay:
            for ax in [ax1, ax2, ax3, ax4]:
                ax.xaxis.set_major_formatter(formatter)
        else:
            for ax in [ax1, ax2, ax3]:
                ax.xaxis.set_major_formatter(formatter)
        
        # Plot one-way delay if available
        if has_one_way_delay:
            delay_df = data["one_way_delay"]
            
            # Plot one-way delay as a single line
            ax4.plot(delay_df["time_receiver"], delay_df["one_way_delay_ms"], 
                    label="One-Way Delay (based on TWCC)",
                    color=colors[0], linestyle='-')
            
            # Add legend for one-way delay
            ax4.legend()
        
        # Add more space for the title
        plt.tight_layout(rect=[0, 0, 1, 0.95])  # Adjust the top margin to make room for the title
        plt.show()

# The plot_experiment_one_way_delay function has been removed as its functionality
# is now fully integrated into the plot_experiment_results function
