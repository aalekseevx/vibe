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

def get_quality_name_from_qualityID(config, test_cases, experiment, qualityID):
    """Get the quality name for a given qualityID from the test case configuration."""
    if experiment not in test_cases:
        return f"Unknown (qualityID: {qualityID})"
    
    test_case = test_cases[experiment]
    sender_config = test_case.get('sender', {})
    
    if sender_config.get('mode') != 'simulcast':
        return f"Default (qualityID: {qualityID})"
    
    # Get tracks with simulcast presets
    tracks = sender_config.get('tracks', [])
    simulcast_configs = config.get('simulcast_configs_presets', {})
    
    # Find the quality in the tracks' simulcast presets
    for track in tracks:
        preset_name = track.get('simulcast_preset')
        if preset_name and preset_name in simulcast_configs:
            preset = simulcast_configs[preset_name]
            qualities = preset.get('qualities', [])
            
            for quality in qualities:
                if quality.get('id') == qualityID:
                    return quality.get('name', f"Unknown (qualityID: {qualityID})")
    
    return f"Unknown (qualityID: {qualityID})"

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
    
    # Get tracks with simulcast presets
    tracks = sender_config.get('tracks', [])
    simulcast_configs = config.get('simulcast_configs_presets', {})
    
    # Find all qualities and their bitrates
    all_qualities = []
    
    # Find qualities in the tracks' simulcast presets
    for track in tracks:
        preset_name = track.get('simulcast_preset')
        if preset_name and preset_name in simulcast_configs:
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
        exp_path = Path(base_path) / experiment
        flow_id = 0  # Hardcoded to flow 0
        logs = {}

        for log_file in exp_path.glob(f"{flow_id}_*.log"):
            parts = log_file.stem.split("_", 1)
            if len(parts) != 2:
                continue  # skip malformed names
            _, log_type = parts
            logs[log_type] = log_file

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
                            "marker", "size", "twcc", "unwrapped_seq", "trackID", "qualityID", "isRTX", "isFEC"
                        ], converters={'isRTX': string_to_bool, 'isFEC': string_to_bool})

                        df["time"] = pd.to_datetime(df["time"], unit="ms")
                    else:
                        df = pd.read_csv(logs[key], header=None, names=["time", "size"])
                        df["time"] = pd.to_datetime(df["time"], unit="ms")
                    flow_data[key] = df

        # Fix isRTX and isFEC flags on receiver side
        if "sender_rtp" in flow_data and "receiver_rtp" in flow_data:
            flow_data["receiver_rtp"] = fix_receiver_rtx_fec_flags(
                flow_data["sender_rtp"], 
                flow_data["receiver_rtp"]
            )

        experiment_data[experiment] = flow_data

    return experiment_data

def fix_receiver_rtx_fec_flags(sender_df, receiver_df):
    """Fix isRTX and isFEC flags on receiver side by using values from sender side.
    
    This function addresses a bug where isRTX and isFEC flags are not set correctly
    on the receiver side. It joins sender and receiver dataframes based on ssrc and
    unwrapped_seq, then updates the receiver flags with the sender values.
    
    Args:
        sender_df: DataFrame containing sender RTP log data
        receiver_df: DataFrame containing receiver RTP log data
        
    Returns:
        Updated receiver DataFrame with corrected isRTX and isFEC flags
    """
    # Create a copy of the receiver dataframe to avoid modifying the original
    receiver_df_fixed = receiver_df.copy()
    
    # Extract only the columns we need from the sender dataframe
    sender_flags = sender_df[['ssrc', 'unwrapped_seq', 'isRTX', 'isFEC']].copy()
    
    # Merge the dataframes on ssrc and unwrapped_seq
    merged = pd.merge(
        receiver_df_fixed,
        sender_flags,
        on=['ssrc', 'unwrapped_seq'],
        how='left',
        suffixes=('_receiver', '_sender')
    )
    
    # Update the receiver flags with the sender values where matches exist
    mask = ~merged['isRTX_sender'].isna()
    receiver_df_fixed.loc[mask, 'isRTX'] = merged.loc[mask, 'isRTX_sender'].astype(bool)
    
    mask = ~merged['isFEC_sender'].isna()
    receiver_df_fixed.loc[mask, 'isFEC'] = merged.loc[mask, 'isFEC_sender'].astype(bool)
    
    return receiver_df_fixed

def compute_bitrates(experiment_data, window_ms=500):
    """Compute bitrates for all experiments, including per-track bitrates and RTX/FEC bitrates."""
    for exp_name, data in experiment_data.items():
        if "sender_rtp" in data:
            df = data["sender_rtp"].copy()
            df["time_bin"] = df["time"].dt.floor(f"{window_ms}ms")
            
            # Compute overall bitrate
            bitrate_df = df.groupby("time_bin")["size"].sum().reset_index()
            bitrate_df["bitrate_kbps"] = (bitrate_df["size"] * 8) / (window_ms / 1000) / 1000
            data["bitrate"] = bitrate_df
            
            track_bitrates = {}
            track_bitrates_df = pd.DataFrame()
            
            all_time_bins = df["time_bin"].unique()
            track_bitrates_df["time_bin"] = all_time_bins
            track_bitrates_df = track_bitrates_df.sort_values("time_bin").reset_index(drop=True)
            
            media_df = df[~df["isRTX"] & ~df["isFEC"]]
            
            for trackID, group in media_df.groupby("trackID"):
                track_df = group.groupby("time_bin")["size"].sum().reset_index()
                track_df["bitrate_kbps"] = (track_df["size"] * 8) / (window_ms / 1000) / 1000
                
                track_bitrates[trackID] = track_df
                
                column_name = f"bitrate_kbps_track_{trackID}"
                track_bitrates_df = pd.merge(
                    track_bitrates_df, 
                    track_df[["time_bin", "bitrate_kbps"]].rename(columns={"bitrate_kbps": column_name}), 
                    on="time_bin", 
                    how="left"
                )
                track_bitrates_df[column_name] = track_bitrates_df[column_name].fillna(0)
            
            data["track_bitrates"] = track_bitrates
            data["track_bitrates_df"] = track_bitrates_df
            
            # Compute RTX bitrate
            rtx_df = df[df["isRTX"]]
            if not rtx_df.empty:
                rtx_bitrate = rtx_df.groupby("time_bin")["size"].sum().reset_index()
                rtx_bitrate["bitrate_kbps"] = (rtx_bitrate["size"] * 8) / (window_ms / 1000) / 1000
                data["rtx_bitrate"] = rtx_bitrate
            
            # Compute FEC bitrate
            fec_df = df[df["isFEC"]]
            if not fec_df.empty:
                fec_bitrate = fec_df.groupby("time_bin")["size"].sum().reset_index()
                fec_bitrate["bitrate_kbps"] = (fec_bitrate["size"] * 8) / (window_ms / 1000) / 1000
                data["fec_bitrate"] = fec_bitrate

def compute_lost_packets(experiment_data):
    """Compute lost packets for all experiments using transport-wide congestion control (TWCC) sequence numbers.
    
    This function calculates cumulative lost packets based on TWCC sequence numbers.
    
    Args:
        experiment_data: Dictionary containing experiment data
    """
    for exp_name, data in experiment_data.items():
        if "receiver_rtp" in data:
            df = data["receiver_rtp"].copy() 
            min_twcc = df["twcc"].min()
            df["expected_packets"] = df["twcc"] - min_twcc + 1
            df["received_packets"] = np.arange(1, len(df) + 1)
            df["lost_packets"] = df["expected_packets"] - df["received_packets"]
            data["lost_packets"] = df[["time", "lost_packets"]].copy()

def compute_interarrival_jitter(experiment_data):
    """Compute interarrival jitter according to RFC 3550, separated by trackID.
    
    RTX and FEC packets are excluded from the jitter calculation.
    """
    for exp_name, data in experiment_data.items():
        if "receiver_rtp" in data:
            df = data["receiver_rtp"].copy()
            df = df[~df["isRTX"] & ~df["isFEC"]]
            
            track_jitter = {}
            
            for trackID, group in df.groupby("trackID"):
                group = group.sort_values("time")
                
                first_arrival = group["time"].min()
                group["arrival_ms"] = (group["time"] - first_arrival).dt.total_seconds() * 1000
                
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
                
                jitter_df = pd.DataFrame({
                    "time": group["time"],
                    "jitter": jitter
                })
                
                track_jitter[trackID] = jitter_df
            
            data["track_jitter"] = track_jitter

def compute_one_way_delay(experiment_data):
    """Compute one-way delay of RTP packets as the difference between timestamps in sender_rtp and receiver_rtp.
    Uses twccNr for matching packets between sender and receiver instead of unwrappedSeqNr."""
    for exp_name, data in experiment_data.items():
        if "sender_rtp" in data and "receiver_rtp" in data:
            sender_df = data["sender_rtp"]
            receiver_df = data["receiver_rtp"]
            
            merged_df = pd.merge(
                sender_df[["time", "trackID", "twcc", "timestamp"]],
                receiver_df[["time", "trackID", "twcc", "timestamp"]],
                on="twcc",
                suffixes=("_sender", "_receiver")
            )
            
            if not merged_df.empty:
                merged_df["one_way_delay_ms"] = (
                    (merged_df["time_receiver"] - merged_df["time_sender"]).dt.total_seconds() * 1000
                )
                
                data["one_way_delay"] = merged_df[["time_receiver", "trackID_sender", "one_way_delay_ms"]]

def create_quality_timeline(experiment_data, config, test_cases):
    """Create a timeline of quality changes for each stream (trackID).
    
    This function tracks all qualityIDs for each trackID and creates a new timeline entry
    whenever the qualityID (and thus quality) changes for a trackID.
    
    RTX and FEC packets are excluded from the timeline.
    """
    for exp_name, data in experiment_data.items():
        if "sender_rtp" in data:
            df = data["sender_rtp"]
            df = df[~df["isRTX"] & ~df["isFEC"]]

            timeline_data = []
            
            for trackID, track_df in df.groupby("trackID"):
                track_df = track_df.sort_values("time")
                
                current_qualityID = None
                segment_start = None
                
                for _, row in track_df.iterrows():
                    time = row["time"]
                    qualityID = row["qualityID"]
                    
                    if current_qualityID is None or qualityID != current_qualityID:
                        if current_qualityID is not None and segment_start is not None:
                            quality_name = get_quality_name_from_qualityID(config, test_cases, exp_name, current_qualityID)
                            
                            timeline_data.append({
                                "trackID": str(trackID),
                                "Quality": quality_name,
                                "Start": segment_start,
                                "Finish": time,
                                "qualityID": current_qualityID
                            })
                        
                        current_qualityID = qualityID
                        segment_start = time
                
                if current_qualityID is not None and segment_start is not None:
                    quality_name = get_quality_name_from_qualityID(config, test_cases, exp_name, current_qualityID)
                    
                    timeline_data.append({
                        "trackID": str(trackID),
                        "Quality": quality_name,
                        "Start": segment_start,
                        "Finish": track_df["time"].max(),
                        "qualityID": current_qualityID
                    })
            
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
        
        if is_simulcast:
            fig, (ax_timeline, ax1, ax4, ax2, ax3) = plt.subplots(5, 1, figsize=(12, 24), 
                                                                gridspec_kw={'height_ratios': [1, 2, 2, 2, 2]})
        else:
            fig, (ax1, ax4, ax2, ax3) = plt.subplots(4, 1, figsize=(12, 20))
        
        fig.suptitle(exp_name, fontsize=16)
        
        locator = mdates.AutoDateLocator()
        formatter = mdates.ConciseDateFormatter(locator)
        
        # Plot quality timeline using matplotlib only for simulcast mode
        if is_simulcast and "quality_timeline" in data:
            timeline_df = data["quality_timeline"]
            
            unique_trackIDs = timeline_df["trackID"].unique()
            unique_qualities = timeline_df["Quality"].unique()
            
            track_cats = {trackID: i+1 for i, trackID in enumerate(unique_trackIDs)}
            
            sorted_qualities = get_quality_bitrates(config, test_cases, exp_name)
            
            # Create a color mapping for qualities based on bitrate level
            quality_colormapping = {}
            for quality in unique_qualities:
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
            
            verts = []
            bar_colors = []
            
            legend_elements = []
            
            for _, row in timeline_df.iterrows():
                trackID = row["trackID"]
                quality = row["Quality"]
                start_time = row["Start"]
                end_time = row["Finish"]
                
                # Calculate polygon height - fixed height regardless of number of tracks
                polygon_height = 0.4  # Fixed height for each polygon
                
                # Create a rectangle for this timeline entry
                v = [
                    (mdates.date2num(start_time), track_cats[trackID] - polygon_height),
                    (mdates.date2num(start_time), track_cats[trackID] + polygon_height),
                    (mdates.date2num(end_time), track_cats[trackID] + polygon_height),
                    (mdates.date2num(end_time), track_cats[trackID] - polygon_height),
                    (mdates.date2num(start_time), track_cats[trackID] - polygon_height)
                ]
                verts.append(v)
                
                # Color is based on quality
                color = quality_colormapping[quality]
                bar_colors.append(color)
                
                # Add to legend if not already there
                from matplotlib.patches import Patch
                if quality not in [e.get_label() for e in legend_elements]:
                    legend_elements.append(Patch(facecolor=color, label=quality))
            
            bars = PolyCollection(verts, facecolors=bar_colors)
            ax_timeline.add_collection(bars)
            ax_timeline.autoscale()
            
            ax_timeline.set_title("Quality Timeline")
            ax_timeline.set_yticks(list(track_cats.values()))
            ax_timeline.set_yticklabels(list(track_cats.keys()))
            
            ax_timeline.legend(handles=legend_elements, loc='upper left')
            
        if is_simulcast:
            ax_timeline.xaxis.set_major_locator(locator)
            ax_timeline.xaxis.set_major_formatter(formatter)
        
        # Setup subplots with titles and labels
        ax1.set_title("Bitrate Utilization")
        ax1.set_ylabel("Bitrate (kbps)")
        
        ax4.set_title("One-Way Delay")
        ax4.set_ylabel("Delay (ms)")
        
        ax2.set_title("Cumulative Lost Packets")
        ax2.set_ylabel("Lost Packets Count")
        
        ax3.set_title("Interarrival Jitter (RFC 3550)")
        ax3.set_ylabel("Jitter (ms)")
        ax3.set_xlabel("Time")
        
        colors = plt.colormaps.get_cmap('tab10').colors

        path_characteristics = path_characteristics_map.get(exp_name)
        if path_characteristics:
            start_time = None
            if "sender_rtp" in data:
                start_time = data["sender_rtp"]["time"].min()
            
            if start_time is not None:
                time_points = [start_time + timedelta(seconds=t) for t in path_characteristics['time']]
                ax1.plot(time_points, path_characteristics['capacity_kbps'], 
                        label="Path Capacity", color='black', linestyle='-.', linewidth=2)

        stacked_df = None
        stack_columns = []
        stack_labels = []
        stack_colors = []
        
        if "track_bitrates_df" in data:
            stacked_df = data["track_bitrates_df"].copy()
            
            track_columns = [col for col in stacked_df.columns if col.startswith("bitrate_kbps_track_")]
            track_labels = [f"Track {col.split('_')[-1]}" for col in track_columns]
            
            stack_columns.extend(track_columns)
            stack_labels.extend(track_labels)
            stack_colors.extend(colors[2:2+len(track_columns)])
        else:
            stacked_df = pd.DataFrame()
            if "sender_rtp" in data:
                df = data["sender_rtp"]
                all_time_bins = df["time_bin"].unique()
                stacked_df["time_bin"] = all_time_bins
                stacked_df = stacked_df.sort_values("time_bin").reset_index(drop=True)
        
        if "rtx_bitrate" in data and not stacked_df.empty:
            rtx_df = data["rtx_bitrate"]
            rtx_column = "bitrate_kbps_rtx"
            
            stacked_df = pd.merge(
                stacked_df,
                rtx_df[["time_bin", "bitrate_kbps"]].rename(columns={"bitrate_kbps": rtx_column}),
                on="time_bin",
                how="left"
            )
            stacked_df[rtx_column] = stacked_df[rtx_column].fillna(0)
            
            stack_columns.append(rtx_column)
            stack_labels.append("RTX")
            stack_colors.append('red')
        
        if "fec_bitrate" in data and not stacked_df.empty:
            fec_df = data["fec_bitrate"]
            fec_column = "bitrate_kbps_fec"
            
            stacked_df = pd.merge(
                stacked_df,
                fec_df[["time_bin", "bitrate_kbps"]].rename(columns={"bitrate_kbps": fec_column}),
                on="time_bin",
                how="left"
            )
            stacked_df[fec_column] = stacked_df[fec_column].fillna(0)
            
            stack_columns.append(fec_column)
            stack_labels.append("FEC")
            stack_colors.append('blue')
        
        if not stacked_df.empty and stack_columns:
            ax1.stackplot(stacked_df["time_bin"], 
                         [stacked_df[col] for col in stack_columns],
                         labels=stack_labels,
                         alpha=0.7,
                         colors=stack_colors)
        
        if "cc_log" in data:
            cc = data["cc_log"]
            ax1.plot(cc["time"], cc["target_bitrate"] / 1000, label="Target Bitrate",
                    color='green', linestyle='--', linewidth=2)
        
        if "lost_packets" in data:
            lost_df = data["lost_packets"]
            
            ax2_left = ax2
            ax2_right = ax2.twinx()
            
            ax2_left.plot(lost_df["time"], lost_df["lost_packets"], 
                    label="Lost Packets",
                    color='blue', linestyle='-', linewidth=2)
            
            ax2_left.set_ylabel("Lost Packets Count")
            
            ax2_right.set_ylabel("Bitrate (kbps)")
            
            if "cc_log" in data:
                cc = data["cc_log"]
                ax2_right.plot(cc["time"], cc["target_bitrate"] / 1000, 
                                label="Target Bitrate",
                                color='green', linestyle='--', linewidth=2)
            
            path_characteristics = path_characteristics_map.get(exp_name)
            if path_characteristics:
                start_time = None
                if "sender_rtp" in data:
                    start_time = data["sender_rtp"]["time"].min()
                
                if start_time is not None:
                    time_points = [start_time + timedelta(seconds=t) for t in path_characteristics['time']]
                    ax2_right.plot(time_points, path_characteristics['capacity_kbps'], 
                                    label="Path Capacity",
                                    color='black', linestyle='-.', linewidth=2)
            
            lines1, labels1 = ax2_left.get_legend_handles_labels()
            lines2, labels2 = ax2_right.get_legend_handles_labels()
            ax2_left.legend(lines1 + lines2, labels1 + labels2, loc='upper left')
        
        if "track_jitter" in data:
            for i, (trackID, jitter_df) in enumerate(data["track_jitter"].items()):
                color = colors[(i + 2) % len(colors)]
                
                ax3.plot(jitter_df["time"], jitter_df["jitter"], 
                        label=f"Track {trackID} Jitter",
                        color=color, linestyle='-')

        ax1.legend(loc='upper left')
        ax3.legend(loc='upper left')
        
        for ax in [ax1, ax2, ax3, ax4]:
            ax.xaxis.set_major_formatter(formatter)
        
        if "one_way_delay" in data and not data["one_way_delay"].empty:
            delay_df = data["one_way_delay"]
            
            avg_delay = delay_df.groupby("time_receiver")["one_way_delay_ms"].mean().reset_index()
            
            ax4_left = ax4
            ax4_right = ax4.twinx()
            
            ax4_left.plot(avg_delay["time_receiver"], avg_delay["one_way_delay_ms"], 
                    label="One-Way Delay",
                    color=colors[0], linestyle='-')
            
            ax4_left.set_ylabel("Delay (ms)")
            
            ax4_right.set_ylabel("Bitrate (kbps)")
            
            if "cc_log" in data:
                cc = data["cc_log"]
                ax4_right.plot(cc["time"], cc["target_bitrate"] / 1000, 
                                label="Target Bitrate",
                                color='green', linestyle='--', linewidth=2)
            
            path_characteristics = path_characteristics_map.get(exp_name)
            if path_characteristics:
                start_time = None
                if "sender_rtp" in data:
                    start_time = data["sender_rtp"]["time"].min()
                
                if start_time is not None:
                    time_points = [start_time + timedelta(seconds=t) for t in path_characteristics['time']]
                    ax4_right.plot(time_points, path_characteristics['capacity_kbps'], 
                                    label="Path Capacity",
                                    color='black', linestyle='-.', linewidth=2)
            
            lines1, labels1 = ax4_left.get_legend_handles_labels()
            lines2, labels2 = ax4_right.get_legend_handles_labels()
            ax4_left.legend(lines1 + lines2, labels1 + labels2, loc='upper left')
        
        plt.tight_layout(rect=[0, 0, 1, 0.95])
        plt.show()
