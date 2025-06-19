-- query_node_exceptions.sql
SELECT
    CAST(start_time AS DATE) AS exception_day,
    COUNT(*) AS exception_count
FROM
    spans
WHERE
    junjo_span_type = 'node'
    AND status_code = 'ERROR'
    AND start_time >= NOW() - INTERVAL '7 days'
GROUP BY
    exception_day
ORDER BY
    exception_day;
