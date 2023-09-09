using Go = import "/go.capnp";

@0xfaa0f0935d698e21;

$Go.package("pkg");
$Go.import("github.com/mikelsr/ww-webcrawler/crawler/proto/pkg");

interface Crawler {
    crawl @0 (url :Text) -> (id :UInt64, urls :List(Text));
    # TODO: result should not be required, add to shared log?
    # Or is the log for avoiding URL repetition?

    addWorker @1 (id :UInt64, worker :Crawler) -> ();
}