/* Copyright (c) 2014-2016 Jason Ish
 * All rights reserved.
 *
 * Redistribution and use in source and binary forms, with or without
 * modification, are permitted provided that the following conditions
 * are met:
 *
 * 1. Redistributions of source code must retain the above copyright
 *    notice, this list of conditions and the following disclaimer.
 * 2. Redistributions in binary form must reproduce the above copyright
 *    notice, this list of conditions and the following disclaimer in the
 *    documentation and/or other materials provided with the distribution.
 *
 * THIS SOFTWARE IS PROVIDED ``AS IS'' AND ANY EXPRESS OR IMPLIED
 * WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF
 * MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
 * DISCLAIMED. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR ANY DIRECT,
 * INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES
 * (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR
 * SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION)
 * HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT,
 * STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING
 * IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
 * POSSIBILITY OF SUCH DAMAGE.
 */
import {Injectable} from "@angular/core";
import {Http} from "@angular/http";
import {TopNavService} from "./topnav.service";
import {AppService} from "./app.service";
import {ConfigService} from "./config.service";
import {ToastrService} from "./toastr.service";
import {ApiService} from "./api.service";

import moment = require("moment");
var queue = require("queue");

export interface ResultSet {
    took:number;
    timedOut:boolean;
    count:number;
    events:any[];
    newestTimestamp?:string;
    oldestTimestamp?:string;
}

export interface AlertGroup {
    count:number,
    escalatedCount:number,
    maxTs:string,
    minTs:string,
    event:any
}

@Injectable()
export class ElasticSearchService {

    private index:string;
    private jobs = queue({concurrency: 4});

    // Default to Logstash/ES 2 template unless we can detect Logstash/ES 5
    // usage.
    public keyword:string = "raw";

    constructor(private api:ApiService,
                private topNavService:TopNavService,
                private appService:AppService,
                private config:ConfigService,
                private toastr:ToastrService) {
        this.index = config.getConfig().ElasticSearchIndex;

        try {
            this.keyword = config.getConfig()["extra"]["elasticSearchKeyword"];
        }
        catch (err) {
        }
        console.log("Use Elastic Search keyword " + this.keyword);
    }

    /**
     * Get the current job size.
     */
    jobSize():number {
        return this.jobs.length;
    }

    search(query:any):Promise<any> {
        return this.api.post("api/1/query", query)
            .then((response:any) => response,
                (error:any) => {
                    throw error.json()
                });
    }

    submit(func:any) {

        let p = new Promise<any>((resolve, reject) => {

            this.jobs.push((cb:any) => {
                func().then(() => {
                    cb();
                    resolve();
                }).catch(() => {
                    cb();
                    reject();
                })
            });

        });

        this.jobs.start();

        return p;
    }

    asKeyword(keyword:string):string {
        return `${keyword}.${this.keyword}`;
    }

    keywordTerm(keyword:string, value:any):any {
        let field = this.asKeyword(keyword);
        let term = {};
        term[field] = value;
        return {
            term: term
        }
    }

    escalateEvent(event:any):Promise<any> {
        event._source.tags.push("escalated");
        event._source.tags.push("evebox.escalated");
        return this.api.post(`api/1/event/${event._id}/escalate`, {})
    }

    deEscalateEvent(event:any):Promise<any> {
        let idx = event._source.tags.indexOf("escalated")
        if (idx > -1) {
            event._source.tags.splice(idx, 1);
        }
        idx = event._source.tags.indexOf("evebox.escalated")
        if (idx > -1) {
            event._source.tags.splice(idx, 1);
        }
        return this.api.post(`api/1/event/${event._id}/de-escalate`, {})
    }

    /**
     * Archive an event.
     *
     * @param event An Elastic Search document.
     */
    archiveEvent(event:any):Promise<any> {
        return this.submit(() => {
            return this.api.post(`api/1/event/${event._id}/archive`, {})
        });
    }

    escalateAlertGroup(alertGroup:AlertGroup):Promise < string > {
        return this.submit(() => {
            let request = {
                signature_id: alertGroup.event._source.alert.signature_id,
                src_ip: alertGroup.event._source.src_ip,
                dest_ip: alertGroup.event._source.dest_ip,
                min_timestamp: alertGroup.minTs,
                max_timestamp: alertGroup.maxTs,
            };
            return this.api.post("api/1/escalate", request);
        });
    }

    archiveAlertGroup(alertGroup:AlertGroup) {
        return this.submit(() => {
            let request = {
                signature_id: alertGroup.event._source.alert.signature_id,
                src_ip: alertGroup.event._source.src_ip,
                dest_ip: alertGroup.event._source.dest_ip,
                min_timestamp: alertGroup.minTs,
                max_timestamp: alertGroup.maxTs,
            };
            return this.api.post("api/1/archive", request);
        });
    }

    removeEscalatedStateFromAlertGroup(alertGroup:AlertGroup):Promise < string > {
        return this.submit(() => {
            let request = {
                alert_group: {
                    signature_id: alertGroup.event._source.alert.signature_id,
                    src_ip: alertGroup.event._source.src_ip,
                    dest_ip: alertGroup.event._source.dest_ip,
                    min_timestamp: alertGroup.minTs,
                    max_timestamp: alertGroup.maxTs,
                },
                tags: ["escalated", "evebox.escalated"],
            };
            return this.api.post("api/1/alert-group/remove-tags", request);
        });

    }

    getEventById(id:string):Promise<any> {
        return this.api.get(`api/1/event/${id}`)
            .then((response:any) => {
                let event = response;

                // Make sure tags exists.
                if (!event._source.tags) {
                    event._source.tags = [];
                }

                return event;
            })
    }

    /**
     * Find events - all events, not just alerts.
     */
    findEvents(options:any = {}):Promise < ResultSet > {

        let queryParts:string[] = [];

        if (options.queryString) {
            queryParts.push(`queryString=${options.queryString}`)
        }
        if (options.timeEnd) {
            queryParts.push(`maxTs=${options.timeEnd}`);
        }
        if (options.timeStart) {
            queryParts.push(`minTs=${options.timeStart}`);
        }
        if (options.eventType && options.eventType != "all") {
            queryParts.push(`eventType=${options.eventType}`)
        }

        let requestOptions:any = {
            search: queryParts.join("&"),
        };

        return this.api.get("api/1/event-query", requestOptions).then((response:any) => {

            let events = response.data;

            events.sort((a:any, b:any) => {
                let x = moment(a._source.timestamp);
                let y = moment(b._source.timestamp);
                return y.diff(x);
            });

            let newestTimestamp:any;
            let oldestTimestamp:any;

            if (events.length > 0) {
                newestTimestamp = events[0]._source["@timestamp"];
                oldestTimestamp = events[events.length - 1]._source["@timestamp"];
            }

            let resultSet:ResultSet = {
                took: response.took,
                count: events.length,
                timedOut: response.timed_out,
                events: events,
                newestTimestamp: newestTimestamp,
                oldestTimestamp: oldestTimestamp
            };

            return resultSet;

        });
    }

    newGetAlerts(options:any = {}):Promise<any> {

        let tags:string[] = [];

        let queryParts:string[] = [];

        if (options.mustHaveTags) {
            options.mustHaveTags.forEach((tag:string) => {
                tags.push(tag);
            })
        }

        if (options.mustNotHaveTags) {
            options.mustNotHaveTags.forEach((tag:string) => {
                tags.push(`-${tag}`);
            })
        }

        queryParts.push(`tags=${tags.join(",")}`);
        queryParts.push(`timeRange=${options.timeRange}`);
        queryParts.push(`queryString=${options.queryString}`);

        let requestOptions = {
            search: queryParts.join("&"),
        };

        return this.api.get("api/1/alerts", requestOptions).then((response:any) => {
            return response.alerts.map((alert:AlertGroup) => {
                return {
                    event: alert,
                    selected: false,
                    date: moment(alert.maxTs).toDate()
                }
            })

        })
    }

    /**
     * Add a time range filter to a query.
     *
     * @param query The query.
     * @param now The time to use as now (a moment object).
     * @param range The time range of the report in seconds.
     */
    addTimeRangeFilter(query:any, now:any, range:number) {
        if (!range) {
            return;
        }

        let then = now.clone().subtract(moment.duration(range, "seconds"));

        query.query.bool.filter.push({
            range: {
                "@timestamp": {
                    gte: `${then.format()}`,
                }
            }
        })
    }

    addSensorNameFilter(query:any, sensor:string) {
        let term = {};
        term[`host.${this.keyword}`] = sensor;
        query.query.bool.filter.push({
            "term": term,
        });
    }

}
