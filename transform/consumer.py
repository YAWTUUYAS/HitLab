"""
Kafka consumer — reads track events and computes the hype score.

hype_score = velocity * 0.40 + geo_spread * 0.35 + replay_rate * 0.25
"""


def compute_hype_score(velocity: float, geo_spread: float, replay_rate: float) -> float:
    # TODO
    pass


def main():
    # TODO: connect to Kafka, consume track-events topic
    # Normalize incoming events
    # Compute hype_score every 5 minutes
    # Write results to TimescaleDB
    pass


if __name__ == "__main__":
    main()
