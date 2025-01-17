import Typography from '@mui/material/Typography';
import React from 'react';
import formatDistanceToNowStrict from 'date-fns/formatDistanceToNowStrict';
import Tooltip from '@mui/material/Tooltip';
import { formatDistanceStrict, intervalToDuration } from 'date-fns';

export const isPermanentBan = (start: Date, end: Date): boolean => {
    const dur = intervalToDuration({
        start,
        end
    });
    const { years } = dur;
    return years != null && years > 5;
};

interface DataTableRelativeDateFieldProps {
    date: Date;
    compareDate?: Date;
    suffix?: boolean;
}

export const DataTableRelativeDateField = ({
    date,
    compareDate,
    suffix = false
}: DataTableRelativeDateFieldProps) => {
    const opts = {
        addSuffix: suffix
    };
    return (
        <Tooltip title={date.toUTCString()}>
            <Typography variant={'body1'}>
                {compareDate
                    ? formatDistanceStrict(date, compareDate, opts)
                    : formatDistanceToNowStrict(date, opts)}
            </Typography>
        </Tooltip>
    );
};
