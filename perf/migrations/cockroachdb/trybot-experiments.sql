


select source_file_id from tracevalues where trace_id='\x0d883d3839a26e1e7b1852fc10795f89' order by commit_number desc limit 5;


select trace_id from tracevalues where source_file_id in (select source_file_id from tracevalues where trace_id='\x0d883d3839a26e1e7b1852fc10795f89' order by commit_number desc limit 5)

select source_file_id from tracevalues where trace_id='\x670f0532af09ebce8c238bf67d99558d' order by commit_number desc limit 5;

select trace_id from tracevalues where trace_id in ('\x670f0532af09ebce8c238bf67d99558d', '\x0d883d3839a26e1e7b1852fc10795f89')


create index by_source_file_id  on TraceValues (source_file_id, trace_id);


select source_file_id from tracevalues where trace_id='\x670f0532af09ebce8c238bf67d99558d' order by commit_number desc limit 5;

select distinct trace_id from tracevalues where source_file_id in ( 615447013218516993, 615303501470105601) limit 5;

-- Start with a single trace, find the source_file_id's of the last 5 commits,
-- and then use those source_file_id's to list all the trace_id's that appear in
-- those files.
select  trace_id from tracevalues@by_source_file_id where source_file_id in (
     select source_file_id from tracevalues@primary where trace_id='\x670f0532af09ebce8c238bf67d99558d' order by commit_number desc limit 5
      ) limit 5;


select source_file from SourceFiles
where souce_file_id in (
    select
        source_file_id
    from
        tracevalues@primary
    where
        trace_id='\x670f0532af09ebce8c238bf67d99558d'
    order by commit_number desc
    limit 5
)

create index by_source_file on SourceFiles (source_file, source_file_id);