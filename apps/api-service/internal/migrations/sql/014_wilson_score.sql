-- Wilson Score lower bound (95% confidence interval)
-- Used for ranking players in casual matches
-- Draws are excluded from calculation (count as 0)
-- Returns a value between 0 and 1

CREATE OR REPLACE FUNCTION wilson_score_lower_bound(
    wins INT,
    losses INT
) RETURNS FLOAT AS $$
DECLARE
    n FLOAT;
    p FLOAT;
    z FLOAT := 1.96;  -- 95% confidence
    z2 FLOAT := 3.8416;  -- z^2
BEGIN
    n := wins + losses;
    IF n = 0 THEN RETURN 0.0; END IF;

    p := wins::FLOAT / n;

    RETURN GREATEST(0,
        ((p + z2 / (2 * n)) - z * SQRT((p * (1 - p) + z2 / (4 * n)) / n))
        / (1 + z2 / n)
    );
END;
$$ LANGUAGE plpgsql IMMUTABLE;

COMMENT ON FUNCTION wilson_score_lower_bound(INT, INT) IS
'Calculates the Wilson Score lower bound (95% CI) for ranking players.
Draws are excluded from the calculation. Returns 0-1 range.
Used for casual match leaderboards.';
