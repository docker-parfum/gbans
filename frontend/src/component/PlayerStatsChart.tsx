import React, { useMemo } from 'react';
import { GlobalTF2StatSnapshot } from '../api';
import {
    Chart as ChartJS,
    CategoryScale,
    LinearScale,
    PointElement,
    LineElement,
    Title,
    Tooltip,
    Legend,
    Filler,
    ChartOptions
} from 'chart.js';
import { Line } from 'react-chartjs-2';
import { renderDateTime } from '../util/text';
import Container from '@mui/material/Container';
import { Colors, ColorsTrans } from '../util/ui';

ChartJS.register(
    CategoryScale,
    LinearScale,
    PointElement,
    LineElement,
    Title,
    Filler,
    Tooltip,
    Legend
);

export interface PlayerStatsChartProps {
    data: GlobalTF2StatSnapshot[];
}

export const PlayerStatsChart = ({ data }: PlayerStatsChartProps) => {
    const options: ChartOptions = {
        responsive: true,
        plugins: {
            legend: {
                position: 'top' as const
            },
            title: {
                display: false,
                text: 'Global TF2 Player Counts'
            }
        }
    };

    const labels = useMemo(() => {
        return data.map((d) => renderDateTime(d.created_on));
    }, [data]);

    const chartData = useMemo(() => {
        return {
            labels,
            datasets: [
                {
                    fill: true,
                    label: 'Players',
                    data: data.map((v) => v.players),
                    borderColor: Colors[0],
                    backgroundColor: ColorsTrans[0]
                },
                {
                    fill: true,
                    label: 'Bots',
                    data: data.map((v) => v.bots),
                    borderColor: Colors[1],
                    backgroundColor: ColorsTrans[1]
                }
            ]
        };
    }, [data, labels]);

    return (
        <Container sx={{ padding: 2 }}>
            {chartData ? <Line options={options} data={chartData} /> : <></>}
        </Container>
    );
};
